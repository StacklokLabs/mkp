// Package identity extracts authenticated user identity from HTTP requests
// and threads it through context for Kubernetes impersonation.
//
// SECURITY NOTE: This package parses JWTs WITHOUT cryptographic signature
// validation. It is designed to run behind a trusted proxy (e.g., ToolHive)
// that has already validated the token. Do NOT use this in contexts where
// the JWT has not been pre-validated by a trusted upstream component.
package identity

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
)

// Identity represents an authenticated user's identity extracted from a JWT.
type Identity struct {
	// User is the username to impersonate in Kubernetes API calls.
	User string
	// Groups are the group memberships to impersonate in Kubernetes API calls.
	Groups []string
}

// contextKey is an unexported type for context keys in this package.
type contextKey struct{}

// identityKey is the context key for storing/retrieving Identity values.
var identityKey = contextKey{}

// WithContext returns a new context with the given Identity stored in it.
func WithContext(ctx context.Context, id *Identity) context.Context {
	return context.WithValue(ctx, identityKey, id)
}

// FromContext extracts the Identity from the context, or returns nil if none is present.
func FromContext(ctx context.Context) *Identity {
	id, _ := ctx.Value(identityKey).(*Identity)
	return id
}

// Config holds configuration for identity extraction from JWTs.
type Config struct {
	// UserClaim is the JWT claim to use for the impersonated username.
	// Defaults to "email".
	UserClaim string
	// GroupsClaim is the JWT claim to use for impersonated groups.
	// Defaults to "groups".
	GroupsClaim string
}

// DefaultConfig returns a Config with default claim names.
func DefaultConfig() *Config {
	return &Config{
		UserClaim:   "email",
		GroupsClaim: "groups",
	}
}

// HTTPContextFunc returns a function suitable for use with mcp-go's
// WithHTTPContextFunc or WithSSEContextFunc. It extracts identity from the
// Authorization header and stores it in the request context.
func HTTPContextFunc(cfg *Config) func(ctx context.Context, r *http.Request) context.Context {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return func(ctx context.Context, r *http.Request) context.Context {
		id, err := ExtractFromRequest(r, cfg)
		if err != nil {
			// Log without the error details to avoid leaking sensitive
			// data from the Authorization header (CodeQL: clear-text logging).
			log.Println("WARN: identity extraction from Authorization header failed")
			return ctx
		}
		if id == nil {
			return ctx
		}
		return WithContext(ctx, id)
	}
}

// ExtractFromRequest extracts an Identity from the HTTP request's Authorization header.
// Returns nil (no error) if no Authorization header is present.
// Returns an error if the header is present but the JWT is malformed or claims are invalid.
func ExtractFromRequest(r *http.Request, cfg *Config) (*Identity, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, nil
	}

	// Expect "Bearer <token>"
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return nil, fmt.Errorf("authorization header is not a Bearer token")
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == "" {
		return nil, fmt.Errorf("empty bearer token")
	}

	return ExtractFromJWT(token, cfg)
}

// ExtractFromJWT extracts an Identity from a JWT token string.
// The JWT is parsed without signature validation (trusted proxy assumption).
func ExtractFromJWT(token string, cfg *Config) (*Identity, error) {
	claims, err := parseJWTPayload(token)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JWT payload: %w", err)
	}

	// Extract user claim
	user, err := extractStringClaim(claims, cfg.UserClaim)
	if err != nil {
		return nil, fmt.Errorf("failed to extract user claim %q: %w", cfg.UserClaim, err)
	}
	if user == "" {
		return nil, fmt.Errorf("user claim %q is empty", cfg.UserClaim)
	}

	// Validate the user is not a Kubernetes system identity.
	// Allowing impersonation of system:* users would be a privilege escalation vector.
	if err := validateIdentity(user); err != nil {
		return nil, err
	}

	// Extract groups claim (optional - not all JWTs include groups)
	groups := extractGroupsClaim(claims, cfg.GroupsClaim)

	if len(groups) > maxGroupCount {
		return nil, fmt.Errorf("number of groups (%d) exceeds maximum of %d", len(groups), maxGroupCount)
	}

	// Validate groups don't include Kubernetes system groups
	for _, group := range groups {
		if err := validateGroup(group); err != nil {
			return nil, err
		}
	}

	return &Identity{
		User:   user,
		Groups: groups,
	}, nil
}

// parseJWTPayload extracts and decodes the payload (second segment) of a JWT.
// It does NOT validate the signature.
func parseJWTPayload(token string) (map[string]interface{}, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT format: expected 3 parts, got %d", len(parts))
	}

	// Decode the payload (second part) - JWT uses base64url encoding without padding
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to base64-decode JWT payload: %w", err)
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JWT payload: %w", err)
	}

	return claims, nil
}

// extractStringClaim extracts a string claim from the JWT claims map.
func extractStringClaim(claims map[string]interface{}, key string) (string, error) {
	val, ok := claims[key]
	if !ok {
		return "", fmt.Errorf("claim not found")
	}

	str, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("claim is not a string (got %T)", val)
	}

	return str, nil
}

// extractGroupsClaim extracts a groups claim from the JWT claims map.
// Groups can be a JSON array of strings. Returns nil if the claim is absent.
func extractGroupsClaim(claims map[string]interface{}, key string) []string {
	val, ok := claims[key]
	if !ok {
		return nil
	}

	// JSON arrays are unmarshaled as []interface{}
	arr, ok := val.([]interface{})
	if !ok {
		return nil
	}

	groups := make([]string, 0, len(arr))
	for _, item := range arr {
		if str, ok := item.(string); ok && str != "" {
			groups = append(groups, str)
		}
	}

	return groups
}

const (
	// maxUserLength is the maximum allowed length for a username claim.
	maxUserLength = 256
	// maxGroupLength is the maximum allowed length for a single group name.
	maxGroupLength = 256
	// maxGroupCount is the maximum number of groups allowed.
	maxGroupCount = 64
)

// rejectedUserPrefixes are Kubernetes system identity prefixes that must not be
// impersonated. Allowing these would let an attacker with a crafted JWT escalate
// to cluster-admin or other privileged system roles.
var rejectedUserPrefixes = []string{
	"system:",
}

// rejectedGroupPrefixes are Kubernetes system group prefixes that must not be
// impersonated.
var rejectedGroupPrefixes = []string{
	"system:",
}

// validateIdentity checks that the user identity is safe to impersonate.
func validateIdentity(user string) error {
	if len(user) > maxUserLength {
		return fmt.Errorf("user identity exceeds maximum length of %d characters", maxUserLength)
	}
	if containsControlCharacters(user) {
		return fmt.Errorf("user identity contains invalid control characters")
	}
	// Kubernetes identity comparison is case-sensitive, so case-sensitive
	// prefix matching is correct here.
	for _, prefix := range rejectedUserPrefixes {
		if strings.HasPrefix(user, prefix) {
			return fmt.Errorf("impersonating Kubernetes system identity %q is not allowed", user)
		}
	}
	return nil
}

// validateGroup checks that a group is safe to impersonate.
func validateGroup(group string) error {
	if len(group) > maxGroupLength {
		return fmt.Errorf("group name exceeds maximum length of %d characters", maxGroupLength)
	}
	if containsControlCharacters(group) {
		return fmt.Errorf("group name contains invalid control characters")
	}
	for _, prefix := range rejectedGroupPrefixes {
		if strings.HasPrefix(group, prefix) {
			return fmt.Errorf("impersonating Kubernetes system group %q is not allowed", group)
		}
	}
	return nil
}

// containsControlCharacters checks for control characters (< 0x20 except space)
// and null bytes that could cause header injection or truncation.
func containsControlCharacters(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}
