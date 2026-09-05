package embedding

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	secutils "github.com/Tencent/WeKnora/internal/utils"
)

const embeddingProxyHelperEnv = "WEKNORA_TEST_EMBEDDING_PROXY_HELPER"

func TestNewEmbeddingHTTPClient_ReusesTransport(t *testing.T) {
	firstTimeout := 15 * time.Second
	secondTimeout := 45 * time.Second
	first := newEmbeddingHTTPClient(firstTimeout)
	second := newEmbeddingHTTPClient(secondTimeout)

	if first == second {
		t.Fatal("expected distinct HTTP clients")
	}
	firstGuard, ok := first.Transport.(*secutils.SSRFValidatingRoundTripper)
	if !ok {
		t.Fatalf("expected SSRF-validating transport, got %T", first.Transport)
	}
	secondGuard, ok := second.Transport.(*secutils.SSRFValidatingRoundTripper)
	if !ok {
		t.Fatalf("expected SSRF-validating transport, got %T", second.Transport)
	}
	if firstGuard.Base != secondGuard.Base {
		t.Fatal("expected embedding HTTP clients to share a base transport")
	}
	if firstGuard.Base != http.RoundTripper(sharedEmbeddingHTTPTransport) {
		t.Fatal("expected embedding HTTP client to use the shared transport")
	}
	if first.Timeout != firstTimeout {
		t.Fatalf("unexpected first client timeout: got %v, want %v", first.Timeout, firstTimeout)
	}
	if second.Timeout != secondTimeout {
		t.Fatalf("unexpected second client timeout: got %v, want %v", second.Timeout, secondTimeout)
	}
}

func TestEmbeddingHTTPTransportProxyEnvironment(t *testing.T) {
	if mode := os.Getenv(embeddingProxyHelperEnv); mode != "" {
		runEmbeddingProxyHelper(t, mode)
		return
	}

	for _, mode := range []string{"https-proxy", "direct"} {
		t.Run(mode, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestEmbeddingHTTPTransportProxyEnvironment$")
			cmd.Env = proxyCleanEnvironment(os.Environ())
			cmd.Env = append(cmd.Env, embeddingProxyHelperEnv+"="+mode)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("proxy helper failed: %v\n%s", err, output)
			}
		})
	}
}

func runEmbeddingProxyHelper(t *testing.T, mode string) {
	base := sharedEmbeddingHTTPTransport.inner.(*http.Transport)
	req, err := http.NewRequest(http.MethodPost, "https://provider.example/v1/embeddings", nil)
	if err != nil {
		t.Fatal(err)
	}

	switch mode {
	case "direct":
		if base.Proxy == nil {
			t.Fatal("embedding transport has no environment proxy resolver")
		}
		proxyURL, err := base.Proxy(req)
		if err != nil {
			t.Fatalf("resolve proxy: %v", err)
		}
		if proxyURL != nil {
			t.Fatalf("expected direct connection with empty proxy environment, got %s", proxyURL)
		}
	case "https-proxy":
		var proxyHits atomic.Int64
		proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			proxyHits.Add(1)
			if r.Method != http.MethodConnect {
				t.Errorf("expected CONNECT through HTTPS proxy, got %s", r.Method)
			}
			http.Error(w, "probe complete", http.StatusBadGateway)
		}))
		defer proxy.Close()
		t.Setenv("HTTPS_PROXY", proxy.URL)

		secutils.SetSSRFWhitelistFromRaw("provider.example")
		t.Cleanup(secutils.ResetSSRFWhitelistForTest)
		ctx, span := withUsageSpan(context.Background())
		req = req.WithContext(ctx)

		client := newEmbeddingHTTPClient(2 * time.Second)
		_, err = client.Do(req)
		if err == nil {
			t.Fatal("expected probe proxy to terminate CONNECT")
		}
		if got := proxyHits.Load(); got != 1 {
			t.Fatalf("expected one HTTPS proxy request, got %d", got)
		}
		if got := span.providerRequests.Load(); got != 1 {
			t.Fatalf("expected one provider_requests count at outbound RoundTrip boundary, got %d", got)
		}
	default:
		t.Fatalf("unknown helper mode %q", mode)
	}
}

func proxyCleanEnvironment(env []string) []string {
	proxyKeys := map[string]struct{}{
		"HTTP_PROXY": {}, "HTTPS_PROXY": {}, "NO_PROXY": {},
		"http_proxy": {}, "https_proxy": {}, "no_proxy": {},
		"REQUEST_METHOD": {},
	}
	clean := make([]string, 0, len(env))
	for _, entry := range env {
		key, _, ok := strings.Cut(entry, "=")
		if _, remove := proxyKeys[key]; ok && remove {
			continue
		}
		clean = append(clean, entry)
	}
	return clean
}

func TestValidateEmbeddingBaseURL_RejectsLoopback(t *testing.T) {
	err := validateEmbeddingBaseURL("http://169.254.169.254/latest/meta-data")
	if err == nil {
		t.Fatal("expected SSRF error for link-local metadata URL")
	}
	if !strings.Contains(err.Error(), "SSRF") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateEmbeddingBaseURL_AllowsEmpty(t *testing.T) {
	if err := validateEmbeddingBaseURL(""); err != nil {
		t.Fatalf("empty base URL should be allowed: %v", err)
	}
}

func TestNewOpenAIEmbedder_RejectsPrivateBaseURL(t *testing.T) {
	_, err := NewOpenAIEmbedder(
		"test-key",
		"http://169.254.169.254/latest/meta-data",
		"text-embedding-3-small",
		511,
		256,
		"model-id",
		nil,
	)
	if err == nil {
		t.Fatal("expected SSRF rejection for link-local metadata URL")
	}
}
