package auth

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/dbohry/huginn/internal/db"
	"github.com/dbohry/huginn/internal/store"
)

func TestHashAndVerify(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Errorf("unexpected hash format: %s", hash)
	}

	ok, err := VerifyPassword("correct horse battery staple", hash)
	if err != nil || !ok {
		t.Errorf("right password: ok=%v err=%v", ok, err)
	}
	ok, err = VerifyPassword("wrong", hash)
	if err != nil || ok {
		t.Errorf("wrong password: ok=%v err=%v", ok, err)
	}
}

func TestHashesAreSalted(t *testing.T) {
	a, _ := HashPassword("same")
	b, _ := HashPassword("same")
	if a == b {
		t.Error("two hashes of the same password are identical, so the salt is not random")
	}
}

func TestVerifyRejectsMalformedHashes(t *testing.T) {
	for _, bad := range []string{
		"", "plain", "$argon2id$", "$bcrypt$v=19$m=1,t=1,p=1$YQ$YQ",
		"$argon2id$v=18$m=19456,t=2,p=1$YQ$YQ",
		"$argon2id$v=19$m=0,t=2,p=1$YQ$YQ",
		"$argon2id$v=19$m=999999999,t=2,p=1$YQ$YQ",
		"$argon2id$v=19$m=19456,t=2,p=1$!!!$YQ",
		"$argon2id$v=19$m=19456,t=2,p=1$YQ$",
	} {
		if ok, err := VerifyPassword("x", bad); ok || err != ErrInvalidHash {
			t.Errorf("%q: ok=%v err=%v, want ErrInvalidHash", bad, ok, err)
		}
	}
}

func TestLimiter(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	l := NewLimiter(3, 10*time.Minute)
	l.now = func() time.Time { return now }

	for i := 0; i < 3; i++ {
		if ok, _ := l.Allow("a"); !ok {
			t.Fatalf("attempt %d blocked too early", i+1)
		}
		l.Fail("a")
		now = now.Add(time.Minute)
	}

	ok, retry := l.Allow("a")
	if ok {
		t.Fatal("should be blocked after 3 failures")
	}
	// The first failure was 3 minutes ago, so it leaves the window in 7.
	if retry != 7*time.Minute {
		t.Errorf("retryAfter = %v, want 7m", retry)
	}
	if ok, _ := l.Allow("other"); !ok {
		t.Error("a different key must not be affected")
	}

	now = now.Add(7 * time.Minute)
	if ok, _ := l.Allow("a"); !ok {
		t.Error("should be allowed again once the oldest failure expires")
	}

	l.Fail("a")
	l.Fail("a")
	l.Reset("a")
	if ok, _ := l.Allow("a"); !ok {
		t.Error("Reset should clear failures")
	}
}

func TestLimiterSweepsStaleKeys(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	l := NewLimiter(5, time.Minute)
	l.now = func() time.Time { return now }

	for i := 0; i < 1100; i++ {
		l.Fail(string(rune('a'+i%26)) + string(rune(i)))
	}
	now = now.Add(2 * time.Minute)
	l.Fail("fresh")
	if len(l.fails) > 2 {
		t.Errorf("stale keys were not swept: %d left", len(l.fails))
	}
}

func TestTempPassword(t *testing.T) {
	re := regexp.MustCompile(`^[a-hjkmnp-z2-9]{4}(-[a-hjkmnp-z2-9]{4}){3}$`)
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		p, err := TempPassword()
		if err != nil {
			t.Fatal(err)
		}
		if !re.MatchString(p) {
			t.Fatalf("unexpected format: %q", p)
		}
		seen[p] = true
	}
	if len(seen) != 50 {
		t.Error("temporary passwords repeat")
	}
}

func TestResetPassword(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := db.Migrate(conn); err != nil {
		t.Fatal(err)
	}
	st := store.New(conn)

	hash, _ := HashPassword("old-password")
	user, err := st.CreateFirstAdmin(ctx, "Alex", hash, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateSession(ctx, "tok", user.ID, false, time.Now(), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}

	temp, err := ResetPassword(ctx, st, "alex") // user names are case-insensitive
	if err != nil {
		t.Fatal(err)
	}

	got, _ := st.UserByID(ctx, user.ID)
	if !got.MustChangePassword {
		t.Error("user must be forced to change the temporary password")
	}
	if ok, _ := VerifyPassword(temp, got.PasswordHash); !ok {
		t.Error("temporary password does not match the stored hash")
	}
	if ok, _ := VerifyPassword("old-password", got.PasswordHash); ok {
		t.Error("old password still works")
	}
	if _, _, err := st.SessionUser(ctx, "tok", time.Now()); err != store.ErrNotFound {
		t.Errorf("old session survived the reset: %v", err)
	}

	if _, err := ResetPassword(ctx, st, "nobody"); err != store.ErrNotFound {
		t.Errorf("unknown user: err = %v, want ErrNotFound", err)
	}
}
