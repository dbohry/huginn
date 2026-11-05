package store

import (
	"context"
	"database/sql/driver"
	"regexp"
	"strings"
	"sync"

	"modernc.org/sqlite"
)

// BlockedWord is a word a user does not want to see in headlines.
type BlockedWord struct {
	ID   int64
	Word string
}

// NormalizeBlockedWord lower-cases a word or phrase and collapses its spaces.
func NormalizeBlockedWord(w string) string {
	return strings.ToLower(strings.Join(strings.Fields(w), " "))
}

// ListBlockedWords returns the user's blocked words, sorted.
func (s *Store) ListBlockedWords(ctx context.Context, userID int64) ([]BlockedWord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, word FROM blocked_words WHERE user_id = ? ORDER BY word`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []BlockedWord{}
	for rows.Next() {
		var b BlockedWord
		if err := rows.Scan(&b.ID, &b.Word); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// AddBlockedWord blocks a word for the user. It returns ErrExists if it is
// already blocked.
func (s *Store) AddBlockedWord(ctx context.Context, userID int64, word string) (BlockedWord, error) {
	word = NormalizeBlockedWord(word)
	res, err := s.db.ExecContext(ctx, `INSERT INTO blocked_words (user_id, word) VALUES (?, ?)`, userID, word)
	if isUnique(err) {
		return BlockedWord{}, ErrExists
	}
	if err != nil {
		return BlockedWord{}, err
	}
	id, err := res.LastInsertId()
	return BlockedWord{ID: id, Word: word}, err
}

// DeleteBlockedWord unblocks one of the user's words.
func (s *Store) DeleteBlockedWord(ctx context.Context, userID, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM blocked_words WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// blockedPattern builds the pattern the SQL function huginn_blocked matches
// headlines against, or "" when the user blocks nothing.
func (s *Store) blockedPattern(ctx context.Context, userID int64) (string, error) {
	words, err := s.ListBlockedWords(ctx, userID)
	if err != nil || len(words) == 0 {
		return "", err
	}
	alts := make([]string, len(words))
	for i, w := range words {
		alts[i] = strings.ReplaceAll(regexp.QuoteMeta(w.Word), " ", `\s+`)
	}
	// A whole word: not preceded or followed by a letter or digit.
	return `(?i)(?:^|[^\p{L}\p{N}])(?:` + strings.Join(alts, "|") + `)(?:[^\p{L}\p{N}]|$)`, nil
}

// notBlocked is an SQL condition that holds for articles whose headline
// (the column title) contains none of the words in the pattern argument. It is
// always true for an empty pattern.
func notBlocked(title string) string {
	return `NOT huginn_blocked(` + title + `, ?)`
}

var patterns sync.Map // pattern -> *regexp.Regexp

func init() {
	sqlite.MustRegisterDeterministicScalarFunction("huginn_blocked", 2,
		func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			pattern, _ := args[1].(string)
			title, _ := args[0].(string)
			if pattern == "" {
				return int64(0), nil
			}
			re, ok := patterns.Load(pattern)
			if !ok {
				c, err := regexp.Compile(pattern)
				if err != nil {
					return nil, err
				}
				re, _ = patterns.LoadOrStore(pattern, c)
			}
			if re.(*regexp.Regexp).MatchString(title) {
				return int64(1), nil
			}
			return int64(0), nil
		})
}
