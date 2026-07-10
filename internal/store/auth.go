package store

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type User struct {
	UserID       string     `json:"user_id"`
	DisplayName  string     `json:"display_name"`
	Email        string     `json:"email,omitempty"`
	PasswordHash string     `json:"-"`
	Role         string     `json:"role"`
	Status       string     `json:"status"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type RegisterUserRequest struct {
	DisplayName string
	Email       string
	Password    string
	Role        string
}

type RefreshTokenRecord struct {
	SessionID string
	UserID    string
	TokenHash string
	UserAgent string
	IPAddress string
	Revoked   bool
	ExpiresAt time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
	RevokedAt sql.NullTime
}

func (s *Store) EnsureRootUser(ctx context.Context, userID string, displayName string, email string, password string) (User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, err
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO app_users (user_id, display_name, email, password_hash, role, status, updated_at)
VALUES ($1,$2,$3,$4,'root','active',now())
ON CONFLICT (user_id) DO UPDATE SET
  display_name = EXCLUDED.display_name,
  email = EXCLUDED.email,
  password_hash = EXCLUDED.password_hash,
  role = 'root',
  status = 'active',
  updated_at = now()`,
		userID, displayName, nullEmpty(email), string(hash),
	)
	if err != nil {
		return User{}, err
	}
	user, _, err := s.GetUserByID(ctx, userID)
	return user, err
}

func (s *Store) RegisterUser(ctx context.Context, req RegisterUserRequest) (User, error) {
	if strings.TrimSpace(req.Email) == "" {
		return User{}, errors.New("email is required")
	}
	if strings.TrimSpace(req.Password) == "" {
		return User{}, errors.New("password is required")
	}
	displayName := strings.TrimSpace(req.DisplayName)
	if displayName == "" {
		displayName = strings.Split(strings.TrimSpace(req.Email), "@")[0]
	}
	role := strings.TrimSpace(req.Role)
	if role == "" {
		role = "user"
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, err
	}
	userID := NewID("user")
	_, err = s.db.ExecContext(ctx, `
INSERT INTO app_users (user_id, display_name, email, password_hash, role, status, updated_at)
VALUES ($1,$2,$3,$4,$5,'active',now())`,
		userID, displayName, normalizeEmail(req.Email), string(hash), role,
	)
	if err != nil {
		return User{}, err
	}
	user, _, err := s.GetUserByID(ctx, userID)
	return user, err
}

func (s *Store) GetUserByID(ctx context.Context, userID string) (User, bool, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT user_id, display_name, COALESCE(email,''), COALESCE(password_hash,''), role, status,
       last_login_at, created_at, updated_at
FROM app_users
WHERE user_id=$1`, userID)
	item, err := scanUser(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, false, nil
		}
		return User{}, false, err
	}
	return item, true, nil
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (User, bool, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT user_id, display_name, COALESCE(email,''), COALESCE(password_hash,''), role, status,
       last_login_at, created_at, updated_at
FROM app_users
WHERE lower(email)=lower($1)`, normalizeEmail(email))
	item, err := scanUser(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, false, nil
		}
		return User{}, false, err
	}
	return item, true, nil
}

func (s *Store) ListUsers(ctx context.Context, role string, status string, limit int) (items []User, err error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	query := `
SELECT user_id, display_name, COALESCE(email,''), COALESCE(password_hash,''), role, status,
       last_login_at, created_at, updated_at
FROM app_users`
	args := []any{}
	clauses := []string{}
	if role = strings.TrimSpace(role); role != "" {
		args = append(args, role)
		clauses = append(clauses, "role=$"+strconv.Itoa(len(args)))
	}
	if status = strings.TrimSpace(status); status != "" {
		args = append(args, status)
		clauses = append(clauses, "status=$"+strconv.Itoa(len(args)))
	}
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	args = append(args, limit)
	query += " ORDER BY created_at DESC LIMIT $" + strconv.Itoa(len(args))

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()
	for rows.Next() {
		item, scanErr := scanUser(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) UpdateLastLogin(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE app_users SET last_login_at=now(), updated_at=now()
WHERE user_id=$1`, userID)
	return err
}

func (s *Store) SetUserPassword(ctx context.Context, userID string, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
UPDATE app_users SET password_hash=$2, updated_at=now()
WHERE user_id=$1`, userID, string(hash))
	return err
}

func (s *Store) SaveRefreshToken(ctx context.Context, record RefreshTokenRecord) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO auth_refresh_tokens (
  session_id, user_id, token_hash, user_agent, ip_address, revoked, expires_at, created_at, updated_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,now(),now())
ON CONFLICT (session_id) DO UPDATE SET
  user_id=EXCLUDED.user_id,
  token_hash=EXCLUDED.token_hash,
  user_agent=EXCLUDED.user_agent,
  ip_address=EXCLUDED.ip_address,
  revoked=EXCLUDED.revoked,
  expires_at=EXCLUDED.expires_at,
  updated_at=now(),
  revoked_at=NULL`,
		record.SessionID, record.UserID, record.TokenHash, record.UserAgent, record.IPAddress, record.Revoked, record.ExpiresAt)
	return err
}

func (s *Store) GetRefreshTokenByHash(ctx context.Context, tokenHash string) (RefreshTokenRecord, bool, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT session_id, user_id, token_hash, COALESCE(user_agent,''), COALESCE(ip_address,''), revoked, expires_at, created_at, updated_at, revoked_at
FROM auth_refresh_tokens
WHERE token_hash=$1`, tokenHash)
	var item RefreshTokenRecord
	if err := row.Scan(&item.SessionID, &item.UserID, &item.TokenHash, &item.UserAgent, &item.IPAddress, &item.Revoked, &item.ExpiresAt, &item.CreatedAt, &item.UpdatedAt, &item.RevokedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RefreshTokenRecord{}, false, nil
		}
		return RefreshTokenRecord{}, false, err
	}
	return item, true, nil
}

func (s *Store) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE auth_refresh_tokens
SET revoked=true, revoked_at=now(), updated_at=now()
WHERE token_hash=$1`, tokenHash)
	return err
}

func (s *Store) RevokeRefreshTokenSession(ctx context.Context, sessionID string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE auth_refresh_tokens
SET revoked=true, revoked_at=now(), updated_at=now()
WHERE session_id=$1`, sessionID)
	return err
}

func VerifyPassword(hash string, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

func normalizeEmail(value string) string {
	return strings.TrimSpace(strings.ToLower(value))
}

type userScanner interface {
	Scan(dest ...any) error
}

func scanUser(row userScanner) (User, error) {
	var item User
	var lastLogin sql.NullTime
	if err := row.Scan(&item.UserID, &item.DisplayName, &item.Email, &item.PasswordHash, &item.Role, &item.Status, &lastLogin, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return User{}, err
	}
	item.Email = normalizeEmail(item.Email)
	if lastLogin.Valid {
		item.LastLoginAt = new(time.Time)
		*item.LastLoginAt = lastLogin.Time
	}
	return item, nil
}
