package server

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/dbohry/huginn/internal/auth"
	"github.com/dbohry/huginn/internal/store"
)

// userSummary is what the Users screen shows about an account.
type userSummary struct {
	ID                 int64  `json:"id"`
	Username           string `json:"username"`
	IsAdmin            bool   `json:"isAdmin"`
	MustChangePassword bool   `json:"mustChangePassword"`
	LastSeenAt         *int64 `json:"lastSeenAt"` // Unix seconds; null if never signed in
}

func toSummary(u store.User) userSummary {
	out := userSummary{ID: u.ID, Username: u.Username, IsAdmin: u.IsAdmin, MustChangePassword: u.MustChangePassword}
	if !u.LastSeenAt.IsZero() {
		t := u.LastSeenAt.Unix()
		out.LastSeenAt = &t
	}
	return out
}

// admin runs h only for an administrator.
func (s *server) admin(h authedHandler) http.Handler {
	return s.authed(false, func(w http.ResponseWriter, r *http.Request, user store.User, sess store.Session) {
		if !user.IsAdmin {
			writeError(w, http.StatusForbidden, "Only administrators can do that.")
			return
		}
		h(w, r, user, sess)
	})
}

func (s *server) listUsers(w http.ResponseWriter, r *http.Request, _ store.User, _ store.Session) {
	users, err := s.store.ListUsers(r.Context())
	if err != nil {
		serverError(w, err)
		return
	}
	out := make([]userSummary, len(users))
	for i, u := range users {
		out[i] = toSummary(u)
	}
	writeJSON(w, http.StatusOK, map[string][]userSummary{"users": out})
}

// createUser adds an account with a random temporary password. The password
// is in the response and nowhere else, so the administrator has to pass it on.
func (s *server) createUser(w http.ResponseWriter, r *http.Request, _ store.User, _ store.Session) {
	var in struct {
		Username string `json:"username"`
		IsAdmin  bool   `json:"isAdmin"`
	}
	if !decode(w, r, &in) {
		return
	}
	if !usernameRE.MatchString(in.Username) {
		writeError(w, http.StatusBadRequest, usernameRule)
		return
	}

	temp, err := auth.TempPassword()
	if err != nil {
		serverError(w, err)
		return
	}
	hash, err := auth.HashPassword(temp)
	if err != nil {
		serverError(w, err)
		return
	}
	user, err := s.store.CreateUser(r.Context(), in.Username, hash, in.IsAdmin, true, s.now())
	if errors.Is(err, store.ErrUsernameTaken) {
		writeError(w, http.StatusConflict, "That username is already taken.")
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"user": toSummary(user), "temporaryPassword": temp})
}

// resetUserPassword gives another user a new temporary password and signs
// them out everywhere. Administrators change their own password from the
// Account page instead.
func (s *server) resetUserPassword(w http.ResponseWriter, r *http.Request, admin store.User, _ store.Session) {
	id, ok := userID(w, r)
	if !ok {
		return
	}
	if id == admin.ID {
		writeError(w, http.StatusBadRequest, "Change your own password from the Account page.")
		return
	}
	if _, err := s.store.UserByID(r.Context(), id); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "User not found.")
		return
	} else if err != nil {
		serverError(w, err)
		return
	}

	temp, err := auth.ResetUserPassword(r.Context(), s.store, id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "User not found.")
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"temporaryPassword": temp})
}

// deleteUser removes another user, along with their apps and sessions. An
// administrator cannot remove themselves, which also means there is always at
// least one administrator left.
func (s *server) deleteUser(w http.ResponseWriter, r *http.Request, admin store.User, _ store.Session) {
	id, ok := userID(w, r)
	if !ok {
		return
	}
	if id == admin.ID {
		writeError(w, http.StatusBadRequest, "You cannot remove your own account.")
		return
	}
	err := s.store.DeleteUser(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "User not found.")
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func userID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusNotFound, "User not found.")
		return 0, false
	}
	return id, true
}
