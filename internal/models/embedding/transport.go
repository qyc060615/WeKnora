package embedding

import (
	"fmt"
	"net/http"
	"time"

	secutils "github.com/Tencent/WeKnora/internal/utils"
)

// sharedEmbeddingHTTPTransport keeps a single SSRF-safe connection pool for
// all embedding clients. Embedders are recreated as model configuration changes,
// but their outbound connections can be safely reused across client instances,
// so the transport (and its keep-alive pool) is built once at package load.
// It is wrapped in usageCountingTransport so every outbound HTTP attempt is
// counted on the per-invocation usage span (see provider_requests).
var sharedEmbeddingHTTPTransport = &usageCountingTransport{
	inner: newEmbeddingHTTPTransport(),
}

func newEmbeddingHTTPTransport() *http.Transport {
	transport := secutils.NewSSRFSafeTransport(secutils.DefaultSSRFSafeHTTPClientConfig())
	transport.Proxy = http.ProxyFromEnvironment
	return transport
}

// usageCountingTransport counts each outbound HTTP attempt on the
// per-invocation usage span. Because it sits at the transport boundary, it
// observes the true number of httpClient.Do attempts — per-input requests and
// WeKnora-implemented retries included — rather than the number of provider
// method invocations.
type usageCountingTransport struct {
	inner http.RoundTripper
}

func (t *usageCountingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	noteEmbeddingProviderRequest(req.Context())
	return t.inner.RoundTrip(req)
}

// validateEmbeddingBaseURL checks that a resolved embedding API base URL is safe
// for outbound requests. Empty URLs are allowed (callers apply provider defaults).
func validateEmbeddingBaseURL(baseURL string) error {
	if baseURL == "" {
		return nil
	}
	if err := secutils.ValidateURLForSSRF(baseURL); err != nil {
		return fmt.Errorf("base URL SSRF check failed: %w", err)
	}
	return nil
}

// newEmbeddingHTTPClient returns an HTTP client with connection-level SSRF
// protection and redirect validation, aligned with internal/models/chat/transport.go.
// All clients share sharedEmbeddingHTTPTransport so keep-alive connections are
// pooled globally, while each keeps its own timeout.
func newEmbeddingHTTPClient(timeout time.Duration) *http.Client {
	cfg := secutils.DefaultSSRFSafeHTTPClientConfig()
	cfg.Timeout = timeout
	return secutils.NewSSRFSafeHTTPClientWithTransport(cfg, sharedEmbeddingHTTPTransport)
}
