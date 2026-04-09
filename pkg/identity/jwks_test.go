package identity

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
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

// testRSAKeyPair generates a fresh RSA key pair for testing.
func testRSAKeyPair(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return key
}

// testECKeyPair generates a fresh ECDSA P-256 key pair for testing.
func testECKeyPair(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	return key
}

// rsaJWKSKey returns a JWKS key entry for the given RSA public key.
func rsaJWKSKey(kid string, pub *rsa.PublicKey) map[string]string {
	return map[string]string{
		"kty": "RSA",
		"use": "sig",
		"kid": kid,
		"alg": "RS256",
		"n":   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
	}
}

// ecJWKSKey returns a JWKS key entry for the given ECDSA public key.
func ecJWKSKey(kid string, pub *ecdsa.PublicKey) map[string]string {
	return map[string]string{
		"kty": "EC",
		"use": "sig",
		"kid": kid,
		"alg": "ES256",
		"crv": "P-256",
		"x":   base64.RawURLEncoding.EncodeToString(pub.X.Bytes()),
		"y":   base64.RawURLEncoding.EncodeToString(pub.Y.Bytes()),
	}
}

// serveJWKS starts an httptest server that serves a JWKS containing the given keys.
func serveJWKS(t *testing.T, keys ...map[string]string) *httptest.Server {
	t.Helper()

	jwks := map[string]any{"keys": keys}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwks)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// signRSAJWT creates an RS256-signed JWT with the given claims.
func signRSAJWT(t *testing.T, key *rsa.PrivateKey, kid string, claims golangJWT.MapClaims) string {
	t.Helper()
	token := golangJWT.NewWithClaims(golangJWT.SigningMethodRS256, claims)
	token.Header["kid"] = kid
	signed, err := token.SignedString(key)
	require.NoError(t, err)
	return signed
}

// signECJWT creates an ES256-signed JWT with the given claims.
func signECJWT(t *testing.T, key *ecdsa.PrivateKey, kid string, claims golangJWT.MapClaims) string {
	t.Helper()
	token := golangJWT.NewWithClaims(golangJWT.SigningMethodES256, claims)
	token.Header["kid"] = kid
	signed, err := token.SignedString(key)
	require.NoError(t, err)
	return signed
}

func TestNewJWKSClient(t *testing.T) {
	key := testRSAKeyPair(t)
	srv := serveJWKS(t, rsaJWKSKey("test-kid", &key.PublicKey))

	client, err := NewJWKSClient(context.Background(), srv.URL)
	require.NoError(t, err)
	defer client.Stop()

	assert.NotNil(t, client)
}

func TestNewJWKSClient_BadURL(t *testing.T) {
	// t.Context() is cancelled when the test ends, so the Register call
	// that blocks on an unreachable URL will be interrupted by the test timeout.
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	_, err := NewJWKSClient(ctx, "http://localhost:1/nonexistent")
	assert.Error(t, err)
}

func TestExtractFromJWT_WithRSAJWKSValidation(t *testing.T) {
	key := testRSAKeyPair(t)
	kid := "rsa-key-1"
	srv := serveJWKS(t, rsaJWKSKey(kid, &key.PublicKey))

	jwksClient, err := NewJWKSClient(context.Background(), srv.URL)
	require.NoError(t, err)
	defer jwksClient.Stop()

	cfg := &Config{
		UserClaim:   "email",
		GroupsClaim: "groups",
		JWKSClient:  jwksClient,
	}

	t.Run("valid signed token", func(t *testing.T) {
		token := signRSAJWT(t, key, kid, golangJWT.MapClaims{
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
		token := signRSAJWT(t, key, kid, golangJWT.MapClaims{
			"email": "juan@stacklok.com",
			"exp":   golangJWT.NewNumericDate(time.Now().Add(-1 * time.Hour)),
		})

		id, err := ExtractFromJWT(token, cfg)
		assert.Error(t, err)
		assert.Nil(t, id)
		assert.Contains(t, err.Error(), "expired")
	})

	t.Run("token without exp is rejected", func(t *testing.T) {
		token := signRSAJWT(t, key, kid, golangJWT.MapClaims{
			"email": "juan@stacklok.com",
		})

		id, err := ExtractFromJWT(token, cfg)
		assert.Error(t, err)
		assert.Nil(t, id)
	})

	t.Run("token signed with wrong key is rejected", func(t *testing.T) {
		wrongKey := testRSAKeyPair(t)
		token := signRSAJWT(t, wrongKey, kid, golangJWT.MapClaims{
			"email": "attacker@evil.com",
			"exp":   golangJWT.NewNumericDate(time.Now().Add(1 * time.Hour)),
		})

		id, err := ExtractFromJWT(token, cfg)
		assert.Error(t, err)
		assert.Nil(t, id)
		assert.Contains(t, err.Error(), "validation failed")
	})

	t.Run("system user still rejected with valid signature", func(t *testing.T) {
		token := signRSAJWT(t, key, kid, golangJWT.MapClaims{
			"email": "system:admin",
			"exp":   golangJWT.NewNumericDate(time.Now().Add(1 * time.Hour)),
		})

		id, err := ExtractFromJWT(token, cfg)
		assert.Error(t, err)
		assert.Nil(t, id)
		assert.Contains(t, err.Error(), "system identity")
	})
}

func TestExtractFromJWT_WithECDSAJWKSValidation(t *testing.T) {
	key := testECKeyPair(t)
	kid := "ec-key-1"
	srv := serveJWKS(t, ecJWKSKey(kid, &key.PublicKey))

	jwksClient, err := NewJWKSClient(context.Background(), srv.URL)
	require.NoError(t, err)
	defer jwksClient.Stop()

	cfg := &Config{
		UserClaim:   "email",
		GroupsClaim: "groups",
		JWKSClient:  jwksClient,
	}

	t.Run("valid ES256 signed token", func(t *testing.T) {
		token := signECJWT(t, key, kid, golangJWT.MapClaims{
			"email":  "juan@stacklok.com",
			"groups": []string{"eng"},
			"exp":    golangJWT.NewNumericDate(time.Now().Add(1 * time.Hour)),
		})

		id, err := ExtractFromJWT(token, cfg)
		require.NoError(t, err)
		assert.Equal(t, "juan@stacklok.com", id.User)
		assert.Equal(t, []string{"eng"}, id.Groups)
	})

	t.Run("expired ES256 token is rejected", func(t *testing.T) {
		token := signECJWT(t, key, kid, golangJWT.MapClaims{
			"email": "juan@stacklok.com",
			"exp":   golangJWT.NewNumericDate(time.Now().Add(-1 * time.Hour)),
		})

		id, err := ExtractFromJWT(token, cfg)
		assert.Error(t, err)
		assert.Nil(t, id)
	})

	t.Run("wrong EC key is rejected", func(t *testing.T) {
		wrongKey := testECKeyPair(t)
		token := signECJWT(t, wrongKey, kid, golangJWT.MapClaims{
			"email": "attacker@evil.com",
			"exp":   golangJWT.NewNumericDate(time.Now().Add(1 * time.Hour)),
		})

		id, err := ExtractFromJWT(token, cfg)
		assert.Error(t, err)
		assert.Nil(t, id)
	})
}

func TestExtractFromJWT_WithMixedKeyTypes(t *testing.T) {
	rsaKey := testRSAKeyPair(t)
	ecKey := testECKeyPair(t)
	rsaKid := "rsa-key"
	ecKid := "ec-key"

	// Serve both RSA and EC keys from the same JWKS endpoint
	srv := serveJWKS(t,
		rsaJWKSKey(rsaKid, &rsaKey.PublicKey),
		ecJWKSKey(ecKid, &ecKey.PublicKey),
	)

	jwksClient, err := NewJWKSClient(context.Background(), srv.URL)
	require.NoError(t, err)
	defer jwksClient.Stop()

	cfg := &Config{
		UserClaim:   "email",
		GroupsClaim: "groups",
		JWKSClient:  jwksClient,
	}

	t.Run("RSA token validates", func(t *testing.T) {
		token := signRSAJWT(t, rsaKey, rsaKid, golangJWT.MapClaims{
			"email": "rsa-user@example.com",
			"exp":   golangJWT.NewNumericDate(time.Now().Add(1 * time.Hour)),
		})

		id, err := ExtractFromJWT(token, cfg)
		require.NoError(t, err)
		assert.Equal(t, "rsa-user@example.com", id.User)
	})

	t.Run("ECDSA token validates", func(t *testing.T) {
		token := signECJWT(t, ecKey, ecKid, golangJWT.MapClaims{
			"email": "ec-user@example.com",
			"exp":   golangJWT.NewNumericDate(time.Now().Add(1 * time.Hour)),
		})

		id, err := ExtractFromJWT(token, cfg)
		require.NoError(t, err)
		assert.Equal(t, "ec-user@example.com", id.User)
	})
}

func TestExtractFromJWT_IssuerAudienceValidation(t *testing.T) {
	key := testRSAKeyPair(t)
	kid := "iss-aud-key"
	srv := serveJWKS(t, rsaJWKSKey(kid, &key.PublicKey))

	jwksClient, err := NewJWKSClient(context.Background(), srv.URL)
	require.NoError(t, err)
	defer jwksClient.Stop()

	t.Run("matching issuer and audience accepted", func(t *testing.T) {
		cfg := &Config{
			UserClaim:  "email",
			JWKSClient: jwksClient,
			Issuer:     "https://auth.example.com",
			Audience:   "mkp-server",
		}

		token := signRSAJWT(t, key, kid, golangJWT.MapClaims{
			"email": "juan@stacklok.com",
			"iss":   "https://auth.example.com",
			"aud":   "mkp-server",
			"exp":   golangJWT.NewNumericDate(time.Now().Add(1 * time.Hour)),
		})

		id, err := ExtractFromJWT(token, cfg)
		require.NoError(t, err)
		assert.Equal(t, "juan@stacklok.com", id.User)
	})

	t.Run("wrong issuer rejected", func(t *testing.T) {
		cfg := &Config{
			UserClaim:  "email",
			JWKSClient: jwksClient,
			Issuer:     "https://auth.example.com",
		}

		token := signRSAJWT(t, key, kid, golangJWT.MapClaims{
			"email": "juan@stacklok.com",
			"iss":   "https://evil.example.com",
			"exp":   golangJWT.NewNumericDate(time.Now().Add(1 * time.Hour)),
		})

		id, err := ExtractFromJWT(token, cfg)
		assert.Error(t, err)
		assert.Nil(t, id)
		assert.Contains(t, err.Error(), "validation failed")
	})

	t.Run("wrong audience rejected", func(t *testing.T) {
		cfg := &Config{
			UserClaim:  "email",
			JWKSClient: jwksClient,
			Audience:   "mkp-server",
		}

		token := signRSAJWT(t, key, kid, golangJWT.MapClaims{
			"email": "juan@stacklok.com",
			"aud":   "other-service",
			"exp":   golangJWT.NewNumericDate(time.Now().Add(1 * time.Hour)),
		})

		id, err := ExtractFromJWT(token, cfg)
		assert.Error(t, err)
		assert.Nil(t, id)
		assert.Contains(t, err.Error(), "validation failed")
	})

	t.Run("issuer and audience not checked when not configured", func(t *testing.T) {
		cfg := &Config{
			UserClaim:  "email",
			JWKSClient: jwksClient,
			// No Issuer/Audience set
		}

		token := signRSAJWT(t, key, kid, golangJWT.MapClaims{
			"email": "juan@stacklok.com",
			"iss":   "https://any-issuer.com",
			"aud":   "any-audience",
			"exp":   golangJWT.NewNumericDate(time.Now().Add(1 * time.Hour)),
		})

		id, err := ExtractFromJWT(token, cfg)
		require.NoError(t, err)
		assert.Equal(t, "juan@stacklok.com", id.User)
	})
}

func TestValidateJWKSURL(t *testing.T) {
	t.Run("HTTPS is accepted", func(t *testing.T) {
		assert.NoError(t, validateJWKSURL("https://auth.example.com/.well-known/jwks.json"))
	})
	t.Run("HTTP localhost is accepted", func(t *testing.T) {
		assert.NoError(t, validateJWKSURL("http://localhost:8080/jwks"))
		assert.NoError(t, validateJWKSURL("http://127.0.0.1:8080/jwks"))
		assert.NoError(t, validateJWKSURL("http://[::1]:8080/jwks"))
	})
	t.Run("HTTP non-localhost is rejected", func(t *testing.T) {
		err := validateJWKSURL("http://auth.example.com/.well-known/jwks.json")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must use HTTPS")
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
