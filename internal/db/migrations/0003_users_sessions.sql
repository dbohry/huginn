-- Accounts. Usernames are ASCII only (enforced by the application), so NOCASE
-- gives case-insensitive uniqueness.
CREATE TABLE users (
    id                   INTEGER PRIMARY KEY,
    username             TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    password_hash        TEXT    NOT NULL,
    is_admin             INTEGER NOT NULL DEFAULT 0 CHECK (is_admin IN (0, 1)),
    must_change_password INTEGER NOT NULL DEFAULT 0 CHECK (must_change_password IN (0, 1)),
    created_at           INTEGER NOT NULL,
    -- Unix seconds of the last sign-in or session use. NULL means never.
    last_seen_at         INTEGER
) STRICT;

-- Sessions. The cookie holds a random token; only its SHA-256 is stored, so a
-- leaked database does not hand out working sessions. Times are Unix seconds.
CREATE TABLE sessions (
    token_hash TEXT    PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    persistent INTEGER NOT NULL CHECK (persistent IN (0, 1)),
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL
) STRICT;

CREATE INDEX sessions_user_id ON sessions (user_id);
CREATE INDEX sessions_expires_at ON sessions (expires_at);
