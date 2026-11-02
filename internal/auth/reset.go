package auth

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"

	"github.com/dbohry/huginn/internal/store"
)

// tempAlphabet leaves out characters that are easy to misread (0/o, 1/l/i).
const tempAlphabet = "abcdefghjkmnpqrstuvwxyz23456789"

// ResetPassword gives the user a new random temporary password and marks it
// as one that must be changed at the next sign-in. It also signs the user out
// everywhere. It returns the temporary password so the caller can show it.
func ResetPassword(ctx context.Context, st *store.Store, username string) (string, error) {
	user, err := st.UserByUsername(ctx, username)
	if err != nil {
		return "", err
	}
	return ResetUserPassword(ctx, st, user.ID)
}

// ResetUserPassword does the same for a user known by id.
func ResetUserPassword(ctx context.Context, st *store.Store, id int64) (string, error) {
	temp, err := TempPassword()
	if err != nil {
		return "", err
	}
	hash, err := HashPassword(temp)
	if err != nil {
		return "", err
	}
	if err := st.SetPassword(ctx, id, hash, true); err != nil {
		return "", err
	}
	if err := st.DeleteUserSessions(ctx, id, ""); err != nil {
		return "", err
	}
	return temp, nil
}

// TempPassword returns a random password such as "kq7m-x3np-b2vh-9wtd"
// (16 characters, about 79 bits).
func TempPassword() (string, error) {
	const groups, perGroup = 4, 4
	out := make([]byte, 0, groups*(perGroup+1))
	max := big.NewInt(int64(len(tempAlphabet)))
	for g := 0; g < groups; g++ {
		if g > 0 {
			out = append(out, '-')
		}
		for i := 0; i < perGroup; i++ {
			n, err := rand.Int(rand.Reader, max)
			if err != nil {
				return "", fmt.Errorf("generate password: %w", err)
			}
			out = append(out, tempAlphabet[n.Int64()])
		}
	}
	return string(out), nil
}
