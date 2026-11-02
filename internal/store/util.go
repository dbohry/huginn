package store

import (
	"database/sql"
	"strings"
	"time"
)

// Times are stored as UTC RFC 3339 text, which sorts correctly as a string.
func formatTime(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func parseTime(s sql.NullString) *time.Time {
	if !s.Valid || s.String == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, s.String)
	if err != nil {
		return nil
	}
	return &t
}

func isUnique(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
