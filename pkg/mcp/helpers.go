package mcp

// BoolPtr returns a pointer to the given bool value
func BoolPtr(b bool) *bool {
	return &b
}