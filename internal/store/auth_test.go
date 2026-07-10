package store

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestUserJSONNeverIncludesPasswordHash(t *testing.T) {
	payload, err := json.Marshal(User{
		UserID:       "user_1",
		DisplayName:  "Candidate",
		Email:        "candidate@example.com",
		PasswordHash: "must-never-cross-the-api-boundary",
		Role:         "user",
		Status:       "active",
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), "password") || strings.Contains(string(payload), "must-never") {
		t.Fatalf("user JSON leaked password material: %s", payload)
	}
}
