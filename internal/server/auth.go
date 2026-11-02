package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/dbohry/huginn/internal/auth"
	"github.com/dbohry/huginn/internal/feed"
	"github.com/dbohry/huginn/internal/refresh"
	"github.com/dbohry/huginn/internal/store"
)

const (
	sessionCookie = "huginn_session"

	// A browser that ticked "Keep me signed in" stays signed in for 30 days
	// after its last visit. Otherwise a session lasts 12 hours after the last
	// visit, and the cookie goes when the browser closes.
	persistentTTL = 30 * 24 * time.Hour
	temporaryTTL  = 12 * time.Hour

	// Sessions are only extended when this much of their lifetime has passed,
	// so ordinary browsing does not write to the database on every request.
	extendAfter = 5 * time.Minute

	maxBody = 4 << 10

	minPasswordChars = 8
	maxPasswordBytes = 128
)

var usernameRE = regexp.MustCompile(`^[A-Za-z0-9._-]{3,32}$`)

const (
	usernameRule = "Username must be 3 to 32 characters: letters, numbers, dots, dashes and underscores."
	passwordRule = "Password must be at least 8 characters."
)

// server holds what the handlers share.
type server struct {
	db      *sql.DB
	store   *store.Store
	version string
	now     func() time.Time

	refresh *refresh.Service
	// bg tracks background work (refreshing feeds after an import, for example).
	bg sync.WaitGroup

	loginByIP     *auth.Limiter
	loginByUser   *auth.Limiter
	passwordByID  *auth.Limiter
	refreshByUser *auth.Limiter

	// dummyHash is checked when a username does not exist, so a failed sign-in
	// takes as long whether or not the account is real.
	dummyHash string
}

func newServer(conn *sql.DB, version string) *server {
	dummy, err := auth.HashPassword("not a real password")
	if err != nil {
		panic(err) // only fails if the system has no random source
	}
	st := store.New(conn)
	return &server{
		db:            conn,
		store:         st,
		version:       version,
		now:           time.Now,
		refresh:       refresh.New(st, feed.NewFetcher(version), slog.Default()),
		loginByIP:     auth.NewLimiter(20, 15*time.Minute),
		loginByUser:   auth.NewLimiter(10, 15*time.Minute),
		passwordByID:  auth.NewLimiter(10, 15*time.Minute),
		refreshByUser: auth.NewLimiter(20, time.Minute),
		dummyHash:     dummy,
	}
}

type userJSON struct {
	ID                 int64  `json:"id"`
	Username           string `json:"username"`
	IsAdmin            bool   `json:"isAdmin"`
	MustChangePassword bool   `json:"mustChangePassword"`
}

func toJSON(u store.User) userJSON {
	return userJSON{ID: u.ID, Username: u.Username, IsAdmin: u.IsAdmin, MustChangePassword: u.MustChangePassword}
}

// --- setup (first run) ---------------------------------------------------

func (s *server) setupStatus(w http.ResponseWriter, r *http.Request) {
	has, err := s.store.HasUsers(r.Context())
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"needsSetup": !has})
}

// setup creates the first account, which is the administrator. It only works
// while no account exists.
func (s *server) setup(w http.ResponseWriter, r *http.Request) {
	var in struct{ Username, Password string }
	if !decode(w, r, &in) {
		return
	}
	if !usernameRE.MatchString(in.Username) {
		writeError(w, http.StatusBadRequest, usernameRule)
		return
	}
	if msg := checkPassword(in.Password); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	// Cheap early exit once setup is done, before spending time on a hash.
	if has, err := s.store.HasUsers(r.Context()); err != nil {
		serverError(w, err)
		return
	} else if has {
		writeError(w, http.StatusConflict, "Setup is already done.")
		return
	}

	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		serverError(w, err)
		return
	}
	user, err := s.store.CreateFirstAdmin(r.Context(), in.Username, hash, s.now())
	if errors.Is(err, store.ErrSetupClosed) {
		writeError(w, http.StatusConflict, "Setup is already done.")
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	if err := s.startSession(w, r, user, true); err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]userJSON{"user": toJSON(user)})
}

// --- sign in and out -----------------------------------------------------

func (s *server) login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Username, Password string
		Remember           bool
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Username == "" || in.Password == "" {
		writeError(w, http.StatusBadRequest, "Enter your username and password.")
		return
	}

	ipKey := clientIP(r)
	userKey := strings.ToLower(in.Username)
	if ok, retry := s.loginByIP.Allow(ipKey); !ok {
		tooManyAttempts(w, retry)
		return
	}
	if ok, retry := s.loginByUser.Allow(userKey); !ok {
		tooManyAttempts(w, retry)
		return
	}

	user, err := s.store.UserByUsername(r.Context(), in.Username)
	hash := s.dummyHash
	switch {
	case err == nil:
		hash = user.PasswordHash
	case !errors.Is(err, store.ErrNotFound):
		serverError(w, err)
		return
	}
	match, verr := auth.VerifyPassword(in.Password, hash)
	if verr != nil {
		serverError(w, verr)
		return
	}
	if err != nil || !match {
		s.loginByIP.Fail(ipKey)
		s.loginByUser.Fail(userKey)
		writeError(w, http.StatusUnauthorized, "Wrong username or password.")
		return
	}

	s.loginByIP.Reset(ipKey)
	s.loginByUser.Reset(userKey)
	if err := s.startSession(w, r, user, in.Remember); err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]userJSON{"user": toJSON(user)})
}

func (s *server) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		if err := s.store.DeleteSession(r.Context(), store.HashToken(c.Value)); err != nil {
			serverError(w, err)
			return
		}
	}
	clearCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
}

// --- the signed-in user --------------------------------------------------

type authedHandler func(w http.ResponseWriter, r *http.Request, user store.User, sess store.Session)

// authed runs h only for a signed-in user. Accounts that still have a
// temporary password are limited to handlers that set allowTemp.
func (s *server) authed(allowTemp bool, h authedHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookie)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "Not signed in.")
			return
		}
		sess, user, err := s.store.SessionUser(r.Context(), store.HashToken(c.Value), s.now())
		if errors.Is(err, store.ErrNotFound) {
			clearCookie(w, r)
			writeError(w, http.StatusUnauthorized, "Not signed in.")
			return
		}
		if err != nil {
			serverError(w, err)
			return
		}
		if user.MustChangePassword && !allowTemp {
			writeJSON(w, http.StatusForbidden, map[string]string{
				"error": "Choose a new password first.",
				"code":  "password_change_required",
			})
			return
		}
		s.extend(r.Context(), w, r, c.Value, sess)
		h(w, r, user, sess)
	})
}

func (s *server) me(w http.ResponseWriter, r *http.Request, user store.User, _ store.Session) {
	writeJSON(w, http.StatusOK, map[string]userJSON{"user": toJSON(user)})
}

func (s *server) changePassword(w http.ResponseWriter, r *http.Request, user store.User, sess store.Session) {
	var in struct{ Current, New string }
	if !decode(w, r, &in) {
		return
	}

	key := fmt.Sprint(user.ID)
	if ok, retry := s.passwordByID.Allow(key); !ok {
		tooManyAttempts(w, retry)
		return
	}
	match, err := auth.VerifyPassword(in.Current, user.PasswordHash)
	if err != nil {
		serverError(w, err)
		return
	}
	if !match {
		s.passwordByID.Fail(key)
		// Not 401: that means "not signed in", and the browser would drop the session screen.
		writeError(w, http.StatusForbidden, "Your current password is wrong.")
		return
	}
	if msg := checkPassword(in.New); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	if in.New == in.Current {
		writeError(w, http.StatusBadRequest, "Choose a different password from the current one.")
		return
	}

	hash, err := auth.HashPassword(in.New)
	if err != nil {
		serverError(w, err)
		return
	}
	if err := s.store.SetPassword(r.Context(), user.ID, hash, false); err != nil {
		serverError(w, err)
		return
	}
	// Every other browser has to sign in again with the new password.
	if err := s.store.DeleteUserSessions(r.Context(), user.ID, sess.TokenHash); err != nil {
		serverError(w, err)
		return
	}
	s.passwordByID.Reset(key)
	w.WriteHeader(http.StatusNoContent)
}

// --- sessions and cookies ------------------------------------------------

func ttl(persistent bool) time.Duration {
	if persistent {
		return persistentTTL
	}
	return temporaryTTL
}

func (s *server) startSession(w http.ResponseWriter, r *http.Request, user store.User, persistent bool) error {
	token, hash, err := store.NewToken()
	if err != nil {
		return err
	}
	now := s.now()
	if err := s.store.CreateSession(r.Context(), hash, user.ID, persistent, now, now.Add(ttl(persistent))); err != nil {
		return err
	}
	// Only used for the "last seen" line on the Users screen.
	_ = s.store.TouchUser(r.Context(), user.ID, now)
	setCookie(w, r, token, persistent)
	return nil
}

// extend pushes a session's expiry forward while it is being used. A failure
// here is not worth failing the request for: the session just expires on the
// old schedule.
func (s *server) extend(ctx context.Context, w http.ResponseWriter, r *http.Request, token string, sess store.Session) {
	// Sessions are only extended every few minutes, which also keeps "last seen" current.
	now := s.now()
	want := now.Add(ttl(sess.Persistent))
	if want.Sub(sess.ExpiresAt) < extendAfter {
		return
	}
	if err := s.store.ExtendSession(ctx, sess.TokenHash, want); err != nil {
		return
	}
	_ = s.store.TouchUser(ctx, sess.UserID, now)
	setCookie(w, r, token, sess.Persistent)
}

// sessionCookieFor returns the session cookie with the attributes every
// version of it shares.
func sessionCookieFor(r *http.Request, value string) *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookie,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   isHTTPS(r),
	}
}

func setCookie(w http.ResponseWriter, r *http.Request, token string, persistent bool) {
	c := sessionCookieFor(r, token)
	if persistent {
		c.MaxAge = int(persistentTTL.Seconds())
	}
	http.SetCookie(w, c)
}

func clearCookie(w http.ResponseWriter, r *http.Request) {
	c := sessionCookieFor(r, "")
	c.MaxAge = -1
	http.SetCookie(w, c)
}

// isHTTPS is true when the browser reached us over TLS, directly or through a
// reverse proxy that says so. Plain-http installs on a home network are
// supported, so the cookie is only marked Secure when it can be.
func isHTTPS(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// --- request protection --------------------------------------------------

// sameOrigin rejects requests that change data when the browser says they
// came from another site. It backs up the SameSite cookie setting.
func sameOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		if site := r.Header.Get("Sec-Fetch-Site"); site == "cross-site" {
			writeError(w, http.StatusForbidden, "Cross-site request refused.")
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			if err != nil || u.Host != r.Host {
				writeError(w, http.StatusForbidden, "Cross-site request refused.")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// --- helpers -------------------------------------------------------------

// decode reads a small JSON body into v. It writes the error response itself
// and returns false when the body is unusable.
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	return decodeLimit(w, r, v, maxBody)
}

// decodeLimit is decode for the few requests that may be larger than usual.
func decodeLimit(w http.ResponseWriter, r *http.Request, v any, limit int64) bool {
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, "Send the request as JSON.")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "That request could not be read.")
		return false
	}
	return true
}

func checkPassword(p string) string {
	if utf8.RuneCountInString(p) < minPasswordChars {
		return passwordRule
	}
	if len(p) > maxPasswordBytes {
		return "Password must be at most 128 bytes."
	}
	return ""
}

func tooManyAttempts(w http.ResponseWriter, retry time.Duration) {
	minutes := int(retry.Minutes()) + 1
	w.Header().Set("Retry-After", fmt.Sprint(int(retry.Seconds())+1))
	unit := "minutes"
	if minutes == 1 {
		unit = "minute"
	}
	writeError(w, http.StatusTooManyRequests, fmt.Sprintf("Too many attempts. Try again in %d %s.", minutes, unit))
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func serverError(w http.ResponseWriter, err error) {
	// The detail goes to the log, not to the browser.
	slog.Error("request failed", "err", err)
	writeError(w, http.StatusInternalServerError, "Something went wrong.")
}
