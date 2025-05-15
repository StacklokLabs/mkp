package ratelimit

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/mock"
)

type mockSession struct {
	id string
}

func (m *mockSession) SessionID() string {
	return m.id
}

func (*mockSession) NotificationChannel() chan<- mcp.JSONRPCNotification {
	return nil
}

func (*mockSession) Initialize() {}

func (*mockSession) Initialized() bool {
	return true
}

// MockToolHandler for testing middleware
type MockToolHandler struct {
	mock.Mock
}

// Handle implements the ToolHandler interface for mock testing
func (m *MockToolHandler) Handle(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(*mcp.CallToolResult), args.Error(1)
}
