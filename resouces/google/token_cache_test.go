package google

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

type countingTokenIssuer struct {
	calls  atomic.Int32
	expiry time.Duration
	delay  time.Duration
}

func (i *countingTokenIssuer) Issue(context.Context, string) (*oauth2.Token, error) {
	call := i.calls.Add(1)
	if i.delay > 0 {
		time.Sleep(i.delay)
	}
	return &oauth2.Token{AccessToken: string(rune('a' + call - 1)), TokenType: "Bearer", Expiry: time.Now().Add(i.expiry)}, nil
}

func TestCachedGoogleTokenIssuerReusesValidToken(t *testing.T) {
	inner := &countingTokenIssuer{expiry: time.Hour}
	issuer := NewCachedGoogleTokenIssuer(inner, 5*time.Minute)
	first, err := issuer.Issue(context.Background(), "USER@vertr.ai")
	if err != nil {
		t.Fatal(err)
	}
	second, err := issuer.Issue(context.Background(), "user@vertr.ai")
	if err != nil {
		t.Fatal(err)
	}
	if first.AccessToken != second.AccessToken || inner.calls.Load() != 1 {
		t.Fatalf("tokens = %q/%q calls = %d", first.AccessToken, second.AccessToken, inner.calls.Load())
	}
}

func TestCachedGoogleTokenIssuerRefreshesNearExpiry(t *testing.T) {
	inner := &countingTokenIssuer{expiry: 4 * time.Minute}
	issuer := NewCachedGoogleTokenIssuer(inner, 5*time.Minute)
	_, _ = issuer.Issue(context.Background(), "user@vertr.ai")
	_, _ = issuer.Issue(context.Background(), "user@vertr.ai")
	if inner.calls.Load() != 2 {
		t.Fatalf("calls = %d", inner.calls.Load())
	}
}

func TestCachedGoogleTokenIssuerCollapsesConcurrentRefreshes(t *testing.T) {
	inner := &countingTokenIssuer{expiry: time.Hour, delay: 20 * time.Millisecond}
	issuer := NewCachedGoogleTokenIssuer(inner, 5*time.Minute)
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := issuer.Issue(context.Background(), "user@vertr.ai"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if inner.calls.Load() != 1 {
		t.Fatalf("calls = %d", inner.calls.Load())
	}
}
