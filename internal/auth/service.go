package auth

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"ai-interview-platform/internal/config"
	"ai-interview-platform/internal/store"
)

type Service struct {
	store         *store.Store
	accessSecret  []byte
	refreshSecret []byte
	accessTTL     time.Duration
	refreshTTL    time.Duration
	issuer        string
}

type TokenPair struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	TokenType        string `json:"token_type"`
	ExpiresIn        int64  `json:"expires_in"`
	RefreshExpiresIn int64  `json:"refresh_expires_in"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RegisterRequest struct {
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Password    string `json:"password"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type AuthenticatedUser struct {
	UserID      string `json:"user_id"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email,omitempty"`
	Role        string `json:"role"`
}

type Claims struct {
	Sub       string `json:"sub"`
	Role      string `json:"role,omitempty"`
	Email     string `json:"email,omitempty"`
	Name      string `json:"name,omitempty"`
	Type      string `json:"typ"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
	ID        string `json:"jti"`
	Iss       string `json:"iss"`
}

type Session struct {
	User       AuthenticatedUser
	SessionID  string
	TokenHash  string
	ExpiresAt  time.Time
	AccessTTL  time.Duration
	RefreshTTL time.Duration
}

func NewService(cfg config.Config, dbStore *store.Store) *Service {
	return &Service{
		store:         dbStore,
		accessSecret:  []byte(cfg.JWTAccessSecret),
		refreshSecret: []byte(cfg.JWTRefreshSecret),
		accessTTL:     time.Duration(cfg.AccessTokenTTLMinutes) * time.Minute,
		refreshTTL:    time.Duration(cfg.RefreshTokenTTLDays) * 24 * time.Hour,
		issuer:        "ai-interview-platform",
	}
}

func (s *Service) BootstrapRoot(ctx context.Context, rootID, displayName, email, password string) (store.User, error) {
	return s.store.EnsureRootUser(ctx, rootID, displayName, email, password)
}

func (s *Service) Register(ctx context.Context, req RegisterRequest) (TokenPair, AuthenticatedUser, error) {
	user, err := s.store.RegisterUser(ctx, store.RegisterUserRequest{
		DisplayName: req.DisplayName,
		Email:       req.Email,
		Password:    req.Password,
		Role:        "user",
	})
	if err != nil {
		return TokenPair{}, AuthenticatedUser{}, err
	}
	return s.issuePair(ctx, user, sessionMeta{})
}

func (s *Service) Login(ctx context.Context, req LoginRequest, userAgent string, ipAddress string) (TokenPair, AuthenticatedUser, error) {
	user, ok, err := s.store.GetUserByEmail(ctx, req.Email)
	if err != nil {
		return TokenPair{}, AuthenticatedUser{}, err
	}
	if !ok {
		return TokenPair{}, AuthenticatedUser{}, errors.New("invalid email or password")
	}
	if user.Status != "active" {
		return TokenPair{}, AuthenticatedUser{}, errors.New("user is disabled")
	}
	if err := store.VerifyPassword(user.PasswordHash, req.Password); err != nil {
		return TokenPair{}, AuthenticatedUser{}, errors.New("invalid email or password")
	}
	if err := s.store.UpdateLastLogin(ctx, user.UserID); err != nil {
		return TokenPair{}, AuthenticatedUser{}, err
	}
	return s.issuePair(ctx, user, sessionMeta{userAgent: userAgent, ipAddress: ipAddress})
}

func (s *Service) Refresh(ctx context.Context, req RefreshRequest, userAgent string, ipAddress string) (TokenPair, AuthenticatedUser, error) {
	claims, err := s.ParseAndVerifyRefreshToken(req.RefreshToken)
	if err != nil {
		return TokenPair{}, AuthenticatedUser{}, err
	}
	if claims.Type != "refresh" || claims.Sub == "" {
		return TokenPair{}, AuthenticatedUser{}, errors.New("refresh token is invalid or expired")
	}
	tokenHash := hashToken(req.RefreshToken)
	record, ok, err := s.store.GetRefreshTokenByHash(ctx, tokenHash)
	if err != nil {
		return TokenPair{}, AuthenticatedUser{}, err
	}
	if !ok || record.Revoked || time.Now().After(record.ExpiresAt) {
		return TokenPair{}, AuthenticatedUser{}, errors.New("refresh token is invalid or expired")
	}
	if record.UserID != claims.Sub {
		return TokenPair{}, AuthenticatedUser{}, errors.New("refresh token does not match user")
	}
	user, ok, err := s.store.GetUserByID(ctx, claims.Sub)
	if err != nil {
		return TokenPair{}, AuthenticatedUser{}, err
	}
	if !ok || user.Status != "active" {
		return TokenPair{}, AuthenticatedUser{}, errors.New("user is disabled")
	}
	if err := s.store.RevokeRefreshTokenSession(ctx, record.SessionID); err != nil {
		return TokenPair{}, AuthenticatedUser{}, err
	}
	return s.issuePair(ctx, user, sessionMeta{userAgent: userAgent, ipAddress: ipAddress})
}

func (s *Service) Logout(ctx context.Context, req RefreshRequest) error {
	if strings.TrimSpace(req.RefreshToken) == "" {
		return nil
	}
	return s.store.RevokeRefreshToken(ctx, hashToken(req.RefreshToken))
}

func (s *Service) Me(ctx context.Context, userID string) (AuthenticatedUser, error) {
	user, ok, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return AuthenticatedUser{}, err
	}
	if !ok {
		return AuthenticatedUser{}, errors.New("user not found")
	}
	if user.Status != "active" {
		return AuthenticatedUser{}, errors.New("user is disabled")
	}
	return toAuthenticatedUser(user), nil
}

func (s *Service) AuthenticateAccessToken(token string) (Claims, error) {
	return s.parseAndVerify(token, s.accessSecret)
}

func (s *Service) ParseAndVerifyRefreshToken(token string) (Claims, error) {
	return s.parseAndVerify(token, s.refreshSecret)
}

func (s *Service) issuePair(ctx context.Context, user store.User, meta sessionMeta) (TokenPair, AuthenticatedUser, error) {
	userView := toAuthenticatedUser(user)
	accessClaims := Claims{
		Sub:       user.UserID,
		Role:      user.Role,
		Email:     user.Email,
		Name:      user.DisplayName,
		Type:      "access",
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(s.accessTTL).Unix(),
		ID:        randomID(),
		Iss:       s.issuer,
	}
	refreshClaims := Claims{
		Sub:       user.UserID,
		Type:      "refresh",
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(s.refreshTTL).Unix(),
		ID:        randomID(),
		Iss:       s.issuer,
	}
	accessToken, err := s.sign(accessClaims, s.accessSecret)
	if err != nil {
		return TokenPair{}, AuthenticatedUser{}, err
	}
	refreshToken, err := s.sign(refreshClaims, s.refreshSecret)
	if err != nil {
		return TokenPair{}, AuthenticatedUser{}, err
	}
	if err := s.store.SaveRefreshToken(ctx, store.RefreshTokenRecord{
		SessionID: refreshClaims.ID,
		UserID:    user.UserID,
		TokenHash: hashToken(refreshToken),
		UserAgent: meta.userAgent,
		IPAddress: meta.ipAddress,
		ExpiresAt: time.Unix(refreshClaims.ExpiresAt, 0),
	}); err != nil {
		return TokenPair{}, AuthenticatedUser{}, err
	}
	return TokenPair{
		AccessToken:      accessToken,
		RefreshToken:     refreshToken,
		TokenType:        "Bearer",
		ExpiresIn:        int64(s.accessTTL.Seconds()),
		RefreshExpiresIn: int64(s.refreshTTL.Seconds()),
	}, userView, nil
}

func (s *Service) sign(claims Claims, secret []byte) (string, error) {
	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	headerRaw, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	payloadRaw, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	unsigned := base64.RawURLEncoding.EncodeToString(headerRaw) + "." + base64.RawURLEncoding.EncodeToString(payloadRaw)
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(unsigned))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return unsigned + "." + signature, nil
}

func (s *Service) parseAndVerify(token string, secret []byte) (Claims, error) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 {
		return Claims{}, errors.New("invalid token")
	}
	unsigned := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(unsigned))
	expected := mac.Sum(nil)
	got, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return Claims{}, errors.New("invalid token signature")
	}
	if !hmac.Equal(expected, got) {
		return Claims{}, errors.New("invalid token signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, errors.New("invalid token payload")
	}
	var claims Claims
	if err := json.NewDecoder(bytes.NewReader(payload)).Decode(&claims); err != nil {
		return Claims{}, errors.New("invalid token payload")
	}
	now := time.Now().Unix()
	if claims.ExpiresAt > 0 && now > claims.ExpiresAt {
		return Claims{}, errors.New("token expired")
	}
	if claims.Iss != "" && claims.Iss != s.issuer {
		return Claims{}, errors.New("invalid issuer")
	}
	return claims, nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return fmt.Sprintf("%x", sum[:])
}

func randomID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

type sessionMeta struct {
	userAgent string
	ipAddress string
}

func toAuthenticatedUser(user store.User) AuthenticatedUser {
	return AuthenticatedUser{
		UserID:      user.UserID,
		DisplayName: user.DisplayName,
		Email:       user.Email,
		Role:        user.Role,
	}
}
