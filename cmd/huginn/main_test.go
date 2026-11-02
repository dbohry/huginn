package main

import (
	"bytes"
	"context"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/dbohry/huginn/internal/auth"
	"github.com/dbohry/huginn/internal/config"
	"github.com/dbohry/huginn/internal/db"
	"github.com/dbohry/huginn/internal/store"
)

// newDataDir makes a data directory holding a database with one user, alex,
// who has a password and a signed-in session.
func newDataDir(t *testing.T) (config.Config, *store.Store) {
	t.Helper()
	cfg := config.Config{DataDir: t.TempDir()}
	conn, err := db.Open(cfg.DBPath())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	if err := db.Migrate(conn); err != nil {
		t.Fatal(err)
	}
	hash, err := auth.HashPassword("alex-password-1")
	if err != nil {
		t.Fatal(err)
	}
	st := store.New(conn)
	ctx := context.Background()
	now := time.Now()
	user, err := st.CreateUser(ctx, "alex", hash, true, false, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateSession(ctx, "token-hash", user.ID, true, now, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	return cfg, st
}

func TestResetPasswordGivesATemporaryPasswordAndSignsTheUserOut(t *testing.T) {
	cfg, st := newDataDir(t)
	var out bytes.Buffer

	if err := resetPassword(cfg, []string{"ALEX"}, &out); err != nil { // usernames ignore case
		t.Fatal(err)
	}

	m := regexp.MustCompile(`Temporary password for ALEX: ([a-z0-9]{4}(?:-[a-z0-9]{4}){3})`).FindStringSubmatch(out.String())
	if m == nil {
		t.Fatalf("no temporary password in the output: %q", out.String())
	}
	temp := m[1]
	if !strings.Contains(out.String(), "signed out everywhere") {
		t.Errorf("the output should say what happened: %q", out.String())
	}

	ctx := context.Background()
	user, err := st.UserByUsername(ctx, "alex")
	if err != nil {
		t.Fatal(err)
	}
	if !user.MustChangePassword {
		t.Error("the user must be made to choose a new password")
	}
	if ok, _ := auth.VerifyPassword(temp, user.PasswordHash); !ok {
		t.Error("the printed password does not work")
	}
	if ok, _ := auth.VerifyPassword("alex-password-1", user.PasswordHash); ok {
		t.Error("the old password still works")
	}
	if _, _, err := st.SessionUser(ctx, "token-hash", time.Now()); err != store.ErrNotFound {
		t.Errorf("the user's session should be gone, got %v", err)
	}
}

func TestResetPasswordRefusesBadInput(t *testing.T) {
	cfg, _ := newDataDir(t)
	var out bytes.Buffer

	if err := resetPassword(cfg, nil, &out); err == nil || !strings.Contains(err.Error(), "usage") {
		t.Errorf("no username: %v", err)
	}
	if err := resetPassword(cfg, []string{"a", "b"}, &out); err == nil {
		t.Error("two usernames should be refused")
	}
	if err := resetPassword(cfg, []string{"nobody"}, &out); err == nil || !strings.Contains(err.Error(), `no user named "nobody"`) {
		t.Errorf("unknown user: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("nothing should be printed on failure, got %q", out.String())
	}
}

func TestResetPasswordDoesNotCreateADatabase(t *testing.T) {
	cfg := config.Config{DataDir: filepath.Join(t.TempDir(), "wrong")}
	err := resetPassword(cfg, []string{"alex"}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "HUGINN_DATA_DIR") {
		t.Fatalf("a wrong data directory should be explained, got %v", err)
	}
	if m, _ := filepath.Glob(cfg.DataDir + "/*"); len(m) != 0 {
		t.Errorf("an empty database was created: %v", m)
	}
}
