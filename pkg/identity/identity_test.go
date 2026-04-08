package identity

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildJWT creates a minimal unsigned JWT with the given claims for testing.
// The signature is a dummy value since we don't validate signatures.
func buildJWT(claims map[string]interface{}) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	payload, _ := json.Marshal(claims)
	payloadB64 := base64.RawURLEncoding.EncodeToString(payload)
	signature := base64.RawURLEncoding.EncodeToString([]byte("fake-signature"))
	return fmt.Sprintf("%s.%s.%s", header, payloadB64, signature)
}

func TestExtractFromJWT_ValidToken(t *testing.T) {
	token := buildJWT(map[string]interface{}{
		"sub":    "00u123abc",
		"email":  "juan@stacklok.com",
		"name":   "Juan Antonio",
		"groups": []string{"eng-debugging", "eng-playground"},
	})

	cfg := DefaultConfig()
	id, err := ExtractFromJWT(token, cfg)

	require.NoError(t, err)
	assert.Equal(t, "juan@stacklok.com", id.User)
	assert.Equal(t, []string{"eng-debugging", "eng-playground"}, id.Groups)
}

func TestExtractFromJWT_CustomClaims(t *testing.T) {
	token := buildJWT(map[string]interface{}{
		"preferred_username": "jdoe",
		"roles":              []string{"admin", "viewer"},
	})

	cfg := &Config{
		UserClaim:   "preferred_username",
		GroupsClaim: "roles",
	}
	id, err := ExtractFromJWT(token, cfg)

	require.NoError(t, err)
	assert.Equal(t, "jdoe", id.User)
	assert.Equal(t, []string{"admin", "viewer"}, id.Groups)
}

func TestExtractFromJWT_NoGroups(t *testing.T) {
	token := buildJWT(map[string]interface{}{
		"email": "juan@stacklok.com",
	})

	cfg := DefaultConfig()
	id, err := ExtractFromJWT(token, cfg)

	require.NoError(t, err)
	assert.Equal(t, "juan@stacklok.com", id.User)
	assert.Nil(t, id.Groups)
}

func TestExtractFromJWT_MissingUserClaim(t *testing.T) {
	token := buildJWT(map[string]interface{}{
		"sub":    "00u123abc",
		"groups": []string{"eng-debugging"},
	})

	cfg := DefaultConfig() // expects "email"
	id, err := ExtractFromJWT(token, cfg)

	assert.Error(t, err)
	assert.Nil(t, id)
	assert.Contains(t, err.Error(), "email")
}

func TestExtractFromJWT_EmptyUserClaim(t *testing.T) {
	token := buildJWT(map[string]interface{}{
		"email": "",
	})

	cfg := DefaultConfig()
	id, err := ExtractFromJWT(token, cfg)

	assert.Error(t, err)
	assert.Nil(t, id)
	assert.Contains(t, err.Error(), "empty")
}

func TestExtractFromJWT_MalformedToken(t *testing.T) {
	tests := []struct {
		name  string
		token string
	}{
		{"empty", ""},
		{"single part", "abc"},
		{"two parts", "abc.def"},
		{"four parts", "a.b.c.d"},
		{"invalid base64 payload", "header.!!!invalid!!!.sig"},
		{"invalid JSON payload", fmt.Sprintf("header.%s.sig",
			base64.RawURLEncoding.EncodeToString([]byte("not-json")))},
	}

	cfg := DefaultConfig()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := ExtractFromJWT(tt.token, cfg)
			assert.Error(t, err)
			assert.Nil(t, id)
		})
	}
}

func TestExtractFromJWT_RejectsSystemUser(t *testing.T) {
	tests := []struct {
		name string
		user string
	}{
		{"system:admin", "system:admin"},
		{"system:serviceaccount:default:sa", "system:serviceaccount:default:sa"},
		{"system:anonymous", "system:anonymous"},
		{"system:masters member", "system:kube-controller-manager"},
	}

	cfg := DefaultConfig()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := buildJWT(map[string]interface{}{
				"email": tt.user,
			})
			id, err := ExtractFromJWT(token, cfg)
			assert.Error(t, err)
			assert.Nil(t, id)
			assert.Contains(t, err.Error(), "system identity")
		})
	}
}

func TestExtractFromJWT_RejectsSystemGroup(t *testing.T) {
	tests := []struct {
		name  string
		group string
	}{
		{"system:masters", "system:masters"},
		{"system:authenticated", "system:authenticated"},
		{"system:unauthenticated", "system:unauthenticated"},
	}

	cfg := DefaultConfig()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := buildJWT(map[string]interface{}{
				"email":  "juan@stacklok.com",
				"groups": []string{tt.group},
			})
			id, err := ExtractFromJWT(token, cfg)
			assert.Error(t, err)
			assert.Nil(t, id)
			assert.Contains(t, err.Error(), "system group")
		})
	}
}

func TestExtractFromRequest_NoAuthHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	cfg := DefaultConfig()

	id, err := ExtractFromRequest(req, cfg)
	assert.NoError(t, err)
	assert.Nil(t, id)
}

func TestExtractFromRequest_NonBearerAuth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	cfg := DefaultConfig()

	id, err := ExtractFromRequest(req, cfg)
	assert.Error(t, err)
	assert.Nil(t, id)
	assert.Contains(t, err.Error(), "not a Bearer token")
}

func TestExtractFromRequest_EmptyBearer(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer ")
	cfg := DefaultConfig()

	id, err := ExtractFromRequest(req, cfg)
	assert.Error(t, err)
	assert.Nil(t, id)
}

func TestExtractFromRequest_ValidBearer(t *testing.T) {
	token := buildJWT(map[string]interface{}{
		"email":  "jane@example.com",
		"groups": []string{"devs"},
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	cfg := DefaultConfig()

	id, err := ExtractFromRequest(req, cfg)
	require.NoError(t, err)
	assert.Equal(t, "jane@example.com", id.User)
	assert.Equal(t, []string{"devs"}, id.Groups)
}

func TestContextRoundTrip(t *testing.T) {
	// No identity in context
	ctx := context.Background()
	assert.Nil(t, FromContext(ctx))

	// Store and retrieve identity
	id := &Identity{User: "test@example.com", Groups: []string{"g1"}}
	ctx = WithContext(ctx, id)

	retrieved := FromContext(ctx)
	require.NotNil(t, retrieved)
	assert.Equal(t, "test@example.com", retrieved.User)
	assert.Equal(t, []string{"g1"}, retrieved.Groups)
}

func TestHTTPContextFunc(t *testing.T) {
	cfg := DefaultConfig()
	fn := HTTPContextFunc(cfg)

	t.Run("no auth header returns original context", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		ctx := context.Background()
		newCtx := fn(ctx, req)
		assert.Nil(t, FromContext(newCtx))
	})

	t.Run("valid auth header injects identity", func(t *testing.T) {
		token := buildJWT(map[string]interface{}{
			"email":  "test@example.com",
			"groups": []string{"team-a"},
		})
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		ctx := context.Background()
		newCtx := fn(ctx, req)

		id := FromContext(newCtx)
		require.NotNil(t, id)
		assert.Equal(t, "test@example.com", id.User)
		assert.Equal(t, []string{"team-a"}, id.Groups)
	})

	t.Run("malformed auth header returns original context", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer not-a-jwt")

		ctx := context.Background()
		newCtx := fn(ctx, req)
		assert.Nil(t, FromContext(newCtx))
	})
}

func TestExtractFromJWT_UserClaimNotString(t *testing.T) {
	token := buildJWT(map[string]interface{}{
		"email": 12345, // numeric, not string
	})

	cfg := DefaultConfig()
	id, err := ExtractFromJWT(token, cfg)
	assert.Error(t, err)
	assert.Nil(t, id)
	assert.Contains(t, err.Error(), "not a string")
}

func TestExtractFromJWT_GroupsClaimNotArray(t *testing.T) {
	token := buildJWT(map[string]interface{}{
		"email":  "juan@stacklok.com",
		"groups": "single-group-string", // string instead of array
	})

	cfg := DefaultConfig()
	id, err := ExtractFromJWT(token, cfg)

	// Groups should be nil (graceful fallback), not error
	require.NoError(t, err)
	assert.Equal(t, "juan@stacklok.com", id.User)
	assert.Nil(t, id.Groups)
}

func TestExtractFromJWT_RejectsControlCharacters(t *testing.T) {
	tests := []struct {
		name string
		user string
	}{
		{"newline", "user\n@example.com"},
		{"carriage return", "user\r@example.com"},
		{"null byte", "user\x00@example.com"},
		{"tab", "user\t@example.com"},
	}

	cfg := DefaultConfig()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := buildJWT(map[string]interface{}{
				"email": tt.user,
			})
			id, err := ExtractFromJWT(token, cfg)
			assert.Error(t, err)
			assert.Nil(t, id)
			assert.Contains(t, err.Error(), "control characters")
		})
	}
}

func TestExtractFromJWT_RejectsOverlongUser(t *testing.T) {
	longUser := strings.Repeat("a", 257) + "@example.com"
	token := buildJWT(map[string]interface{}{
		"email": longUser,
	})

	cfg := DefaultConfig()
	id, err := ExtractFromJWT(token, cfg)
	assert.Error(t, err)
	assert.Nil(t, id)
	assert.Contains(t, err.Error(), "maximum length")
}

func TestExtractFromJWT_RejectsTooManyGroups(t *testing.T) {
	groups := make([]string, 65)
	for i := range groups {
		groups[i] = fmt.Sprintf("group-%d", i)
	}
	token := buildJWT(map[string]interface{}{
		"email":  "user@example.com",
		"groups": groups,
	})

	cfg := DefaultConfig()
	id, err := ExtractFromJWT(token, cfg)
	assert.Error(t, err)
	assert.Nil(t, id)
	assert.Contains(t, err.Error(), "exceeds maximum")
}

func TestValidateIdentity_AllowsNormalUsers(t *testing.T) {
	tests := []string{
		"juan@stacklok.com",
		"admin@example.com",
		"user-with-system-in-name",
		"mysystem-user",
	}

	for _, user := range tests {
		t.Run(user, func(t *testing.T) {
			assert.NoError(t, validateIdentity(user))
		})
	}
}

func TestValidateGroup_AllowsNormalGroups(t *testing.T) {
	tests := []string{
		"eng-debugging",
		"eng-playground",
		"admin-group",
		"my-system-group",
	}

	for _, group := range tests {
		t.Run(group, func(t *testing.T) {
			assert.NoError(t, validateGroup(group))
		})
	}
}
