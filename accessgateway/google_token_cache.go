package accessgateway

import (
	"context"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/sync/singleflight"
)

type cachedGoogleTokenIssuer struct {
	inner         GoogleTokenIssuer
	refreshBefore time.Duration
	mu            sync.RWMutex
	tokens        map[string]*oauth2.Token
	requests      singleflight.Group
}

func NewCachedGoogleTokenIssuer(inner GoogleTokenIssuer, refreshBefore time.Duration) GoogleTokenIssuer {
	if refreshBefore < 0 {
		refreshBefore = 0
	}
	return &cachedGoogleTokenIssuer{inner: inner, refreshBefore: refreshBefore, tokens: make(map[string]*oauth2.Token)}
}

func (i *cachedGoogleTokenIssuer) Issue(ctx context.Context, email string) (*oauth2.Token, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if token := i.cached(email, time.Now()); token != nil {
		return token, nil
	}
	value, err, _ := i.requests.Do(email, func() (any, error) {
		if token := i.cached(email, time.Now()); token != nil {
			return token, nil
		}
		token, err := i.inner.Issue(ctx, email)
		if err != nil {
			return nil, err
		}
		if token != nil && token.AccessToken != "" && !token.Expiry.IsZero() {
			copy := *token
			i.mu.Lock()
			i.tokens[email] = &copy
			i.mu.Unlock()
		}
		return cloneOAuthToken(token), nil
	})
	if err != nil {
		return nil, err
	}
	return cloneOAuthToken(value.(*oauth2.Token)), nil
}

func (i *cachedGoogleTokenIssuer) cached(email string, now time.Time) *oauth2.Token {
	i.mu.RLock()
	token := i.tokens[email]
	i.mu.RUnlock()
	if token == nil || token.AccessToken == "" || token.Expiry.IsZero() || !token.Expiry.After(now.Add(i.refreshBefore)) {
		return nil
	}
	return cloneOAuthToken(token)
}

func cloneOAuthToken(token *oauth2.Token) *oauth2.Token {
	if token == nil {
		return nil
	}
	copy := *token
	return &copy
}
