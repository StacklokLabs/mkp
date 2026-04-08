package identity

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	golangJWT "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testKeyPair generates a fresh RSA key pair for testing.
func testKeyPair(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return key
}

// serveJWKS starts an httptest server that serves a JWKS containing the given key.
func serveJWKS(t *testing.T, kid string, pub *rsa.PublicKey) *httptest.Server {
	t.Helper()

	jwks := jwksResponse{
		Keys: []jwkKey{
			{
				Kty: "RSA",
				Use: "sig",
				Kid: kid,
				Alg: "RS256",
				N:   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
				E:   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwks)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// signJWT creates a signed JWT with the given claims.
func signJWT(t *testing.T, key *rsa.PrivateKey, kid string, claims golangJWT.MapClaims) string {
	t.Helper()
	token := golangJWT.NewWithClaims(golangJWT.SigningMethodRS256, claims)
	token.Header["kid"] = kid
	signed, err := token.SignedString(key)
	require.NoError(t, err)
	return signed
}

func TestNewJWKSClient(t *testing.T) {
	key := testKeyPair(t)
	srv := serveJWKS(t, "test-kid", &key.PublicKey)

	client, err := NewJWKSClient(srv.URL)
	require.NoError(t, err)
	defer client.Stop()

	// Should have fetched the key
	pubKey, err := client.GetKey("test-kid")
	require.NoError(t, err)
	assert.NotNil(t, pubKey)
}

func TestNewJWKSClient_BadURL(t *testing.T) {
	_, err := NewJWKSClient("http://localhost:1/nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "initial JWKS fetch")
}

func TestJWKSClient_UnknownKid(t *testing.T) {
	key := testKeyPair(t)
	srv := serveJWKS(t, "known-kid", &key.PublicKey)

	client, err := NewJWKSClient(srv.URL)
	require.NoError(t, err)
	defer client.Stop()

	_, err = client.GetKey("unknown-kid")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestExtractFromJWT_WithJWKSValidation(t *testing.T) {
	key := testKeyPair(t)
	kid := "my-key-1"
	srv := serveJWKS(t, kid, &key.PublicKey)

	jwksClient, err := NewJWKSClient(srv.URL)
	require.NoError(t, err)
	defer jwksClient.Stop()

	cfg := &Config{
		UserClaim:   "email",
		GroupsClaim: "groups",
		JWKSClient:  jwksClient,
	}

	t.Run("valid signed token", func(t *testing.T) {
		token := signJWT(t, key, kid, golangJWT.MapClaims{
			"email":  "juan@stacklok.com",
			"groups": []string{"eng-debugging"},
			"exp":    golangJWT.NewNumericDate(time.Now().Add(1 * time.Hour)),
		})

		id, err := ExtractFromJWT(token, cfg)
		require.NoError(t, err)
		assert.Equal(t, "juan@stacklok.com", id.User)
		assert.Equal(t, []string{"eng-debugging"}, id.Groups)
	})

	t.Run("expired token is rejected", func(t *testing.T) {
		token := signJWT(t, key, kid, golangJWT.MapClaims{
			"email": "juan@stacklok.com",
			"exp":   golangJWT.NewNumericDate(time.Now().Add(-1 * time.Hour)),
		})

		id, err := ExtractFromJWT(token, cfg)
		assert.Error(t, err)
		assert.Nil(t, id)
		assert.Contains(t, err.Error(), "expired")
	})

	t.Run("token without exp is rejected", func(t *testing.T) {
		token := signJWT(t, key, kid, golangJWT.MapClaims{
			"email": "juan@stacklok.com",
		})

		id, err := ExtractFromJWT(token, cfg)
		assert.Error(t, err)
		assert.Nil(t, id)
	})

	t.Run("token signed with wrong key is rejected", func(t *testing.T) {
		wrongKey := testKeyPair(t)
		token := signJWT(t, wrongKey, kid, golangJWT.MapClaims{
			"email": "attacker@evil.com",
			"exp":   golangJWT.NewNumericDate(time.Now().Add(1 * time.Hour)),
		})

		id, err := ExtractFromJWT(token, cfg)
		assert.Error(t, err)
		assert.Nil(t, id)
		assert.Contains(t, err.Error(), "validation failed")
	})

	t.Run("token with unknown kid is rejected", func(t *testing.T) {
		token := signJWT(t, key, "unknown-kid", golangJWT.MapClaims{
			"email": "juan@stacklok.com",
			"exp":   golangJWT.NewNumericDate(time.Now().Add(1 * time.Hour)),
		})

		id, err := ExtractFromJWT(token, cfg)
		assert.Error(t, err)
		assert.Nil(t, id)
	})

	t.Run("system user still rejected with valid signature", func(t *testing.T) {
		token := signJWT(t, key, kid, golangJWT.MapClaims{
			"email": "system:admin",
			"exp":   golangJWT.NewNumericDate(time.Now().Add(1 * time.Hour)),
		})

		id, err := ExtractFromJWT(token, cfg)
		assert.Error(t, err)
		assert.Nil(t, id)
		assert.Contains(t, err.Error(), "system identity")
	})
}

func TestExtractFromJWT_WithoutJWKSValidation(t *testing.T) {
	// When JWKSClient is nil, unsigned tokens still work (trusted proxy mode)
	cfg := DefaultConfig()
	assert.Nil(t, cfg.JWKSClient)

	token := buildJWT(map[string]interface{}{
		"email":  "juan@stacklok.com",
		"groups": []string{"eng"},
	})

	id, err := ExtractFromJWT(token, cfg)
	require.NoError(t, err)
	assert.Equal(t, "juan@stacklok.com", id.User)
}
