package embedding

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRedisEmbeddingCacheGetSetManyAndTTL(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	cache := NewRedisEmbeddingCache(client)
	ctx := context.Background()

	want := map[string][]byte{"k1": {1, 2, 3}, "k2": {4, 5, 6}}
	if err := cache.SetMany(ctx, want, time.Hour); err != nil {
		t.Fatal(err)
	}
	got, err := cache.GetMany(ctx, []string{"k1", "missing", "k2"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("values = %v, want %v", got, want)
	}
	server.FastForward(time.Hour + time.Second)
	got, err = cache.GetMany(ctx, []string{"k1", "k2"})
	if err != nil || len(got) != 0 {
		t.Fatalf("expired values = %v, error=%v", got, err)
	}
}
