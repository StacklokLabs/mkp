package identity

import (
	"context"
	"fmt"
	"net/url"
	"time"

	golangJWT "github.com/golang-jwt/jwt/v5"
	"github.com/lestrrat-go/httprc/v3"
	"github.com/lestrrat-go/jwx/v3/jwk"
)

const (
	// jwksInitTimeout is the maximum time to wait for the initial JWKS fetch
	// during client initialization. This prevents blocking indefinitely if the
	// JWKS endpoint is unreachable.
	jwksInitTimeout = 10 * time.Second

	// jwksLookupTimeout bounds the time for a cache lookup that may trigger
	// an on-demand refresh (e.g. after key rotation). Normally the lookup
	// returns from the in-memory cache instantly.
	jwksLookupTimeout = 5 * time.Second
)

// JWKSClient fetches and caches JSON Web Key Sets from a remote endpoint.
// It delegates to lestrrat-go/jwx for key parsing, caching, background refresh,
// and concurrent request coalescing. Supports RSA, ECDSA, and EdDSA key types.
type JWKSClient struct {
	cache  *jwk.Cache
	url    string
	cancel context.CancelFunc
}

// NewJWKSClient creates a new JWKS client that fetches keys from the given URL.
// The URL must use HTTPS, except for localhost addresses which allow HTTP for
// development. Call Stop() to clean up the background goroutines.
func NewJWKSClient(ctx context.Context, jwksURL string) (*JWKSClient, error) {
	if err := validateJWKSURL(jwksURL); err != nil {
		return nil, err
	}

	// Create a derived context so Stop() can cancel the background refresh
	// independently of the parent context.
	ctx, cancel := context.WithCancel(ctx)

	cache, err := jwk.NewCache(ctx, httprc.NewClient())
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create JWKS cache: %w", err)
	}

	// Use a timeout for the initial registration and fetch to fail fast
	// if the JWKS endpoint is unreachable, rather than blocking indefinitely.
	initCtx, initCancel := context.WithTimeout(ctx, jwksInitTimeout)
	defer initCancel()

	if err := cache.Register(initCtx, jwksURL); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to register JWKS URL %s: %w", jwksURL, err)
	}

	return &JWKSClient{cache: cache, url: jwksURL, cancel: cancel}, nil
}

// Keyfunc returns a jwt.Keyfunc suitable for use with golangJWT.Parse.
// It looks up the key by kid from the cached JWKS and exports the raw
// crypto key (RSA, ECDSA, or EdDSA) for signature verification.
func (c *JWKSClient) Keyfunc() golangJWT.Keyfunc {
	return func(token *golangJWT.Token) (interface{}, error) {
		kid, ok := token.Header["kid"].(string)
		if !ok || kid == "" {
			return nil, fmt.Errorf("JWT header missing 'kid' (key ID)")
		}

		// Use a bounded timeout rather than a long-lived context. The cache
		// stores keys in memory so this is almost always instant; the timeout
		// is a safety net if an on-demand refresh is triggered.
		lookupCtx, lookupCancel := context.WithTimeout(
			context.Background(), jwksLookupTimeout,
		)
		defer lookupCancel()

		keyset, err := c.cache.Lookup(lookupCtx, c.url)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch JWKS: %w", err)
		}

		key, found := keyset.LookupKeyID(kid)
		if !found {
			return nil, fmt.Errorf("key ID %q not found in JWKS", kid)
		}

		var rawKey interface{}
		if err := jwk.Export(key, &rawKey); err != nil {
			return nil, fmt.Errorf("failed to export JWK to raw key: %w", err)
		}

		return rawKey, nil
	}
}

// Stop stops the background JWKS refresh goroutines.
func (c *JWKSClient) Stop() {
	c.cancel()
}

// validateJWKSURL checks that the JWKS URL uses HTTPS. Plain HTTP is only
// permitted for localhost addresses (127.0.0.1, ::1, localhost) to support
// development and testing. Fetching JWKS over unencrypted HTTP in production
// would allow MITM injection of arbitrary signing keys.
func validateJWKSURL(jwksURL string) error {
	u, err := url.Parse(jwksURL)
	if err != nil {
		return fmt.Errorf("invalid JWKS URL: %w", err)
	}

	if u.Scheme == "https" {
		return nil
	}

	host := u.Hostname()
	if u.Scheme == "http" &&
		(host == "localhost" || host == "127.0.0.1" || host == "::1") {
		return nil
	}

	return fmt.Errorf(
		"JWKS URL must use HTTPS (got %s://%s); "+
			"plain HTTP is only allowed for localhost",
		u.Scheme, u.Host,
	)
}
