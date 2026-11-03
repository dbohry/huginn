-- Topics for a user's feeds (a feed belongs to at most one, '' meaning none).
ALTER TABLE feeds ADD COLUMN category TEXT NOT NULL DEFAULT '';

-- Whether the user has been offered the starter feeds (taken them or declined).
-- Accounts that already exist are marked done: the offer is for new accounts.
ALTER TABLE users ADD COLUMN starter_done INTEGER NOT NULL DEFAULT 0 CHECK (starter_done IN (0, 1));
UPDATE users SET starter_done = 1;

-- The list offered to new users. Administrators edit it; it is filled with a
-- built-in list the first time the server starts.
CREATE TABLE starter_feeds (
    id       INTEGER PRIMARY KEY,
    category TEXT NOT NULL,
    title    TEXT NOT NULL,
    url      TEXT NOT NULL UNIQUE,
    position INTEGER NOT NULL DEFAULT 0
) STRICT;

CREATE TABLE settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
) STRICT;
