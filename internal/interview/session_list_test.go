package interview

import (
	"context"
	"testing"
)

func TestListSessionsRequiresUserBoundary(t *testing.T) {
	service := &Service{}
	if _, err := service.ListSessions(context.Background(), "", "", 20); err == nil {
		t.Fatal("ListSessions() should reject an empty user_id before querying storage")
	}
}
