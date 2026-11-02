package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

var (
	// ErrSetupClosed means the first account already exists.
	ErrSetupClosed = errors.New("setup is already done")
	// ErrUsernameTaken means another account already has that username.
	ErrUsernameTaken = errors.New("username is taken")
)

// User is an account.
type User struct {
	ID                 int64
	Username           string
	PasswordHash       string
	IsAdmin            bool
	MustChangePassword bool
	CreatedAt          time.Time
	// LastSeenAt is the zero time when the user has never signed in.
	LastSeenAt time.Time
}

const userColumns = `id, username, password_hash, is_admin, must_change_password, created_at, last_seen_at`

func scanUser(row interface{ Scan(...any) error }) (User, error) {
	var u User
	var created int64
	var seen sql.NullInt64
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.IsAdmin, &u.MustChangePassword, &created, &seen)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}
	u.CreatedAt = time.Unix(created, 0).UTC()
	if seen.Valid {
		u.LastSeenAt = time.Unix(seen.Int64, 0).UTC()
	}
	return u, nil
}

// HasUsers reports whether at least one account exists.
func (s *Store) HasUsers(ctx context.Context) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM users)`).Scan(&exists)
	return exists, err
}

// CreateFirstAdmin creates the administrator account, but only while no
// account exists. The check and the insert are one statement, so two
// simultaneous requests cannot both succeed.
func (s *Store) CreateFirstAdmin(ctx context.Context, username, passwordHash string, now time.Time) (User, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO users (username, password_hash, is_admin, created_at)
		 SELECT ?, ?, 1, ? WHERE NOT EXISTS (SELECT 1 FROM users)`,
		username, passwordHash, now.Unix())
	if err != nil {
		return User{}, fmt.Errorf("create first admin: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return User{}, err
	}
	if n == 0 {
		return User{}, ErrSetupClosed
	}
	return s.UserByUsername(ctx, username)
}

// ListUsers returns every account, oldest first.
func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+userColumns+` FROM users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// CreateUser adds an account. It returns ErrUsernameTaken when another account
// already has the username, ignoring case.
func (s *Store) CreateUser(ctx context.Context, username, passwordHash string, isAdmin, mustChange bool, now time.Time) (User, error) {
	u, err := scanUser(s.db.QueryRowContext(ctx,
		`INSERT INTO users (username, password_hash, is_admin, must_change_password, created_at)
		 VALUES (?, ?, ?, ?, ?) RETURNING `+userColumns,
		username, passwordHash, isAdmin, mustChange, now.Unix()))
	if err != nil {
		if _, lookup := s.UserByUsername(ctx, username); lookup == nil {
			return User{}, ErrUsernameTaken
		}
		return User{}, fmt.Errorf("create user: %w", err)
	}
	return u, nil
}

// DeleteUser removes an account. Its feeds and sessions go with it.
func (s *Store) DeleteUser(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// TouchUser records that the user was just active.
func (s *Store) TouchUser(ctx context.Context, id int64, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE users SET last_seen_at = ? WHERE id = ?`, now.Unix(), id)
	return err
}

// UserByUsername looks an account up ignoring case.
func (s *Store) UserByUsername(ctx context.Context, username string) (User, error) {
	return scanUser(s.db.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE username = ?`, username))
}

// UserByID looks an account up by its id.
func (s *Store) UserByID(ctx context.Context, id int64) (User, error) {
	return scanUser(s.db.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE id = ?`, id))
}

// SetPassword replaces a user's password hash. mustChange says whether the
// user has to pick a new password at the next sign-in.
func (s *Store) SetPassword(ctx context.Context, id int64, passwordHash string, mustChange bool) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE users SET password_hash = ?, must_change_password = ? WHERE id = ?`, passwordHash, mustChange, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// Session is a signed-in browser.
type Session struct {
	TokenHash  string
	UserID     int64
	Persistent bool
	ExpiresAt  time.Time
}

// NewToken returns a random session token for the cookie, and the hash that
// is stored in the database in its place.
func NewToken() (token, hash string, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("generate token: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	return token, HashToken(token), nil
}

// HashToken is the value stored for a cookie token.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// CreateSession stores a new session that expires at expiresAt.
func (s *Store) CreateSession(ctx context.Context, tokenHash string, userID int64, persistent bool, now, expiresAt time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (token_hash, user_id, persistent, created_at, expires_at) VALUES (?, ?, ?, ?, ?)`,
		tokenHash, userID, persistent, now.Unix(), expiresAt.Unix())
	return err
}

// SessionUser returns the session and its user, if the session exists and has
// not expired.
func (s *Store) SessionUser(ctx context.Context, tokenHash string, now time.Time) (Session, User, error) {
	var sess Session
	var expires int64
	row := s.db.QueryRowContext(ctx,
		`SELECT s.token_hash, s.user_id, s.persistent, s.expires_at,
		        u.id, u.username, u.password_hash, u.is_admin, u.must_change_password, u.created_at, u.last_seen_at
		   FROM sessions s JOIN users u ON u.id = s.user_id
		  WHERE s.token_hash = ? AND s.expires_at > ?`, tokenHash, now.Unix())

	var u User
	var created int64
	var seen sql.NullInt64
	err := row.Scan(&sess.TokenHash, &sess.UserID, &sess.Persistent, &expires,
		&u.ID, &u.Username, &u.PasswordHash, &u.IsAdmin, &u.MustChangePassword, &created, &seen)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, User{}, ErrNotFound
	}
	if err != nil {
		return Session{}, User{}, err
	}
	sess.ExpiresAt = time.Unix(expires, 0).UTC()
	u.CreatedAt = time.Unix(created, 0).UTC()
	if seen.Valid {
		u.LastSeenAt = time.Unix(seen.Int64, 0).UTC()
	}
	return sess, u, nil
}

// ExtendSession moves a session's expiry later.
func (s *Store) ExtendSession(ctx context.Context, tokenHash string, expiresAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE sessions SET expires_at = ? WHERE token_hash = ?`, expiresAt.Unix(), tokenHash)
	return err
}

// DeleteSession signs one browser out.
func (s *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
	return err
}

// DeleteUserSessions signs a user out everywhere except the session whose
// hash is keep. Pass "" to sign out every session.
func (s *Store) DeleteUserSessions(ctx context.Context, userID int64, keep string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ? AND token_hash <> ?`, userID, keep)
	return err
}

// PurgeExpiredSessions deletes sessions that have expired.
func (s *Store) PurgeExpiredSessions(ctx context.Context, now time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, now.Unix())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
