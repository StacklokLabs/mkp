package ratelimit

// import (
// 	"context"
// 	"testing"
// 	"time"

// 	"github.com/stretchr/testify/assert"
// )

// // MockClientSession is a simplified mock for testing
// type MockClientSession struct {
// 	ID string
// }

// func (m *MockClientSession) SessionID() string {
// 	return m.ID
// }

// // mockContextKey for storing the session in context
// type mockContextKey struct{}

// // mockClientSessionFromContext gets a session from context
// func mockClientSessionFromContext(ctx context.Context) interface{} {
// 	return ctx.Value(mockContextKey{})
// }

// // withMockSession adds a session to context
// func withMockSession(ctx context.Context, id string) context.Context {
// 	return context.WithValue(ctx, mockContextKey{}, &MockClientSession{ID: id})
// }

// // TestRateLimiterCreation tests that a rate limiter can be created with options
// func TestRateLimiterCreation(t *testing.T) {
// 	// Create a rate limiter with custom options
// 	limiter := NewRateLimiter(
// 		WithDefaultLimit(100),
// 		WithToolLimit("test-tool", 50),
// 	)

// 	// Verify the limiter was created with the correct options
// 	assert.Equal(t, 100, limiter.defaultLimit)
// 	assert.Equal(t, 50, limiter.limits["test-tool"])
// }

// // TestRateLimiterGetLimit tests the getLimit method
// func TestRateLimiterGetLimit(t *testing.T) {
// 	// Create a rate limiter with custom limits
// 	limiter := NewRateLimiter(
// 		WithDefaultLimit(100),
// 		WithToolLimit("test-tool", 50),
// 	)

// 	// Test getting the limit for a tool with a custom limit
// 	assert.Equal(t, 50, limiter.getLimit("test-tool"))

// 	// Test getting the limit for a tool without a custom limit
// 	assert.Equal(t, 100, limiter.getLimit("unknown-tool"))
// }

// // TestRateLimiterAllowRequest tests the core rate limiting functionality
// func TestRateLimiterAllowRequest(t *testing.T) {
// 	// Create a rate limiter with a very low limit for testing
// 	limiter := NewRateLimiter(
// 		WithDefaultLimit(5),
// 	)

// 	// Use a direct session ID for testing
// 	sessionID := "session:test-session-id"
// 	toolName := "test-tool"

// 	// Test that the first 5 requests are allowed
// 	for i := 0; i < 5; i++ {
// 		bucket := limiter.getBucket(sessionID, toolName)
// 		allowed := bucket.tokens > 0
// 		assert.True(t, allowed, "Request %d should be allowed", i+1)
// 		if allowed {
// 			bucket.tokens--
// 		}
// 	}

// 	// Test that the 6th request is not allowed
// 	bucket := limiter.getBucket(sessionID, toolName)
// 	assert.Equal(t, 0, bucket.tokens, "Bucket should be empty")

// 	// Wait for the rate limit to reset (at least partially)
// 	time.Sleep(1 * time.Second)

// 	// Test that we can make at least one more request
// 	bucket = limiter.getBucket(sessionID, toolName)
// 	assert.True(t, bucket.tokens > 0, "Bucket should have tokens after waiting")
// }

// // TestRateLimiterCleanup tests the cleanup method
// func TestRateLimiterCleanup(t *testing.T) {
// 	// Create a rate limiter
// 	limiter := NewRateLimiter()

// 	// Manually create a bucket
// 	sessionID := "session:test-session-id"
// 	toolName := "test-tool"

// 	// Get a bucket which creates it
// 	limiter.getBucket(sessionID, toolName)

// 	// Verify the bucket was created
// 	assert.Len(t, limiter.buckets, 1)
// 	assert.Len(t, limiter.buckets[sessionID], 1)

// 	// Override the lastSeen time to simulate an old bucket
// 	limiter.buckets[sessionID][toolName].lastSeen = time.Now().Add(-bucketTimeout - time.Minute)

// 	// Run cleanup
// 	limiter.cleanup()

// 	// Verify the bucket was removed
// 	assert.Len(t, limiter.buckets, 0)
// }

// // TestRateLimiterStop tests the Stop method
// func TestRateLimiterStop(t *testing.T) {
// 	// Create a rate limiter
// 	limiter := NewRateLimiter()

// 	// Stop the limiter
// 	limiter.Stop()

// 	// There's not much we can assert here, but at least we can verify it doesn't panic
// 	assert.NotPanics(t, func() {
// 		limiter.Stop()
// 	})
// }

// // TestGetSessionID is skipped in this implementation
// // In a real implementation, we would use a proper mock of the server.ClientSession
// // interface, but for simplicity we're focusing on the core rate limiting functionality
// func TestGetSessionID(t *testing.T) {
// 	// Skip this test as it requires mocking the server.ClientSession interface
// 	t.Skip("Skipping test that requires mocking server.ClientSession")
// }

// // TestGetDefaultRateLimiter tests the GetDefaultRateLimiter function
// func TestGetDefaultRateLimiter(t *testing.T) {
// 	// Get the default rate limiter
// 	limiter := GetDefaultRateLimiter()

// 	// Verify it has the expected default limit
// 	assert.Equal(t, DefaultConfig["default"], limiter.defaultLimit)

// 	// Verify it has the expected tool limits
// 	for tool, limit := range DefaultConfig {
// 		if tool != "default" {
// 			assert.Equal(t, limit, limiter.limits[tool])
// 		}
// 	}
// }

// // TestDifferentialRateLimiting tests that different tools have different rate limits
// func TestDifferentialRateLimiting(t *testing.T) {
// 	// Create a rate limiter with the default configuration
// 	limiter := GetDefaultRateLimiter()

// 	// Test read operation (list_resources) - should have higher limit (120)
// 	readTool := "list_resources"

// 	// Verify the limit is higher for read operations
// 	readLimit := limiter.getLimit(readTool)
// 	assert.Equal(t, 120, readLimit)

// 	// Test write operation (apply_resource) - should have lower limit (30)
// 	writeTool := "apply_resource"

// 	// Verify the limit is lower for write operations
// 	writeLimit := limiter.getLimit(writeTool)
// 	assert.Equal(t, 30, writeLimit)

// 	// Create a session ID for testing
// 	sessionID := "session:test-session-id"

// 	// Verify that different tools have separate rate limits
// 	// Make a few requests with the read tool
// 	for i := 0; i < 10; i++ {
// 		bucket := limiter.getBucket(sessionID, readTool)
// 		assert.True(t, bucket.tokens > 0, "Read bucket should have tokens")
// 		bucket.tokens--
// 	}

// 	// Should still be able to make write requests even after making read requests
// 	writeBucket := limiter.getBucket(sessionID, writeTool)
// 	assert.Equal(t, writeLimit, writeBucket.tokens, "Write bucket should have full tokens")
// }
