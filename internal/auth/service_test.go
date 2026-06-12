package auth

import (
	"testing"
	"time"
)

func TestAccessTokenSignAndVerify(t *testing.T) {
	service := &Service{
		accessSecret: []byte("test-access-secret"),
		issuer:       "ai-interview-platform",
	}
	token, err := service.sign(Claims{
		Sub:       "user_1",
		Role:      "user",
		Type:      "access",
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(time.Minute).Unix(),
		ID:        "token_1",
		Iss:       service.issuer,
	}, service.accessSecret)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	claims, err := service.AuthenticateAccessToken(token)
	if err != nil {
		t.Fatalf("verify token: %v", err)
	}
	if claims.Sub != "user_1" || claims.Type != "access" || claims.Role != "user" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestExpiredTokenIsRejected(t *testing.T) {
	service := &Service{
		accessSecret: []byte("test-access-secret"),
		issuer:       "ai-interview-platform",
	}
	token, err := service.sign(Claims{
		Sub:       "user_1",
		Type:      "access",
		IssuedAt:  time.Now().Add(-2 * time.Minute).Unix(),
		ExpiresAt: time.Now().Add(-time.Minute).Unix(),
		ID:        "token_1",
		Iss:       service.issuer,
	}, service.accessSecret)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	if _, err := service.AuthenticateAccessToken(token); err == nil {
		t.Fatal("expected expired token to be rejected")
	}
}

func TestWrongSecretIsRejected(t *testing.T) {
	signer := &Service{
		accessSecret: []byte("test-access-secret"),
		issuer:       "ai-interview-platform",
	}
	verifier := &Service{
		accessSecret: []byte("other-secret"),
		issuer:       "ai-interview-platform",
	}
	token, err := signer.sign(Claims{
		Sub:       "user_1",
		Type:      "access",
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(time.Minute).Unix(),
		ID:        "token_1",
		Iss:       signer.issuer,
	}, signer.accessSecret)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	if _, err := verifier.AuthenticateAccessToken(token); err == nil {
		t.Fatal("expected token signed with another secret to be rejected")
	}
}
