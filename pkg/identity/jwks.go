package identity

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"
)

// defaultJWKSRefreshInterval is how often the JWKS key set is refreshed.
const defaultJWKSRefreshInterval = 5 * time.Minute

// JWKSClient fetches and caches JSON Web Key Sets from a remote endpoint.
// It is used to validate JWT signatures when MKP is deployed without a
// trusted proxy that pre-validates tokens.
type JWKSClient struct {
	url      string
	client   *http.Client
	mu       sync.RWMutex
	keys     map[string]*rsa.PublicKey // kid -> public key
	stopCh   chan struct{}
	stopOnce sync.Once
}

// NewJWKSClient creates a new JWKS client that fetches keys from the given URL.
// It performs an initial fetch and starts a background goroutine to refresh
// keys periodically. Call Stop() to clean up.
func NewJWKSClient(jwksURL string) (*JWKSClient, error) {
	c := &JWKSClient{
		url:    jwksURL,
		client: &http.Client{Timeout: 10 * time.Second},
		keys:   make(map[string]*rsa.PublicKey),
		stopCh: make(chan struct{}),
	}

	// Initial fetch — fail fast if the JWKS endpoint is unreachable
	if err := c.refresh(); err != nil {
		return nil, fmt.Errorf("initial JWKS fetch from %s failed: %w", jwksURL, err)
	}

	// Background refresh
	go c.refreshLoop()

	return c, nil
}

// GetKey returns the RSA public key for the given key ID.
// Returns an error if the key ID is not found.
func (c *JWKSClient) GetKey(kid string) (*rsa.PublicKey, error) {
	c.mu.RLock()
	key, ok := c.keys[kid]
	c.mu.RUnlock()
	if !ok {
		// Try a refresh in case keys were rotated
		if err := c.refresh(); err != nil {
			return nil, fmt.Errorf("JWKS refresh failed: %w", err)
		}
		c.mu.RLock()
		key, ok = c.keys[kid]
		c.mu.RUnlock()
		if !ok {
			return nil, fmt.Errorf("key ID %q not found in JWKS", kid)
		}
	}
	return key, nil
}

// Stop stops the background refresh goroutine.
func (c *JWKSClient) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
}

func (c *JWKSClient) refreshLoop() {
	ticker := time.NewTicker(defaultJWKSRefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := c.refresh(); err != nil {
				// Log but continue — use cached keys
				fmt.Printf("WARN: JWKS refresh failed: %v\n", err)
			}
		case <-c.stopCh:
			return
		}
	}
}

func (c *JWKSClient) refresh() error {
	resp, err := c.client.Get(c.url)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code %d from JWKS endpoint", resp.StatusCode)
	}

	var jwks jwksResponse
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return fmt.Errorf("failed to decode JWKS response: %w", err)
	}

	newKeys := make(map[string]*rsa.PublicKey)
	for _, key := range jwks.Keys {
		if key.Kty != "RSA" || key.Use != "sig" {
			continue
		}
		pubKey, err := parseRSAPublicKey(key)
		if err != nil {
			continue // Skip malformed keys
		}
		newKeys[key.Kid] = pubKey
	}

	if len(newKeys) == 0 {
		return fmt.Errorf("no usable RSA signing keys found in JWKS response")
	}

	c.mu.Lock()
	c.keys = newKeys
	c.mu.Unlock()

	return nil
}

// jwksResponse represents the JSON Web Key Set response.
type jwksResponse struct {
	Keys []jwkKey `json:"keys"`
}

// jwkKey represents a single JSON Web Key.
type jwkKey struct {
	Kty string `json:"kty"` // Key type (RSA, EC, etc.)
	Use string `json:"use"` // Key use (sig, enc)
	Kid string `json:"kid"` // Key ID
	Alg string `json:"alg"` // Algorithm (RS256, etc.)
	N   string `json:"n"`   // RSA modulus (base64url)
	E   string `json:"e"`   // RSA exponent (base64url)
}

// parseRSAPublicKey converts a JWK to an *rsa.PublicKey.
func parseRSAPublicKey(key jwkKey) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(key.N)
	if err != nil {
		return nil, fmt.Errorf("failed to decode modulus: %w", err)
	}

	eBytes, err := base64.RawURLEncoding.DecodeString(key.E)
	if err != nil {
		return nil, fmt.Errorf("failed to decode exponent: %w", err)
	}

	n := new(big.Int).SetBytes(nBytes)
	e := new(big.Int).SetBytes(eBytes)

	return &rsa.PublicKey{
		N: n,
		E: int(e.Int64()),
	}, nil
}
