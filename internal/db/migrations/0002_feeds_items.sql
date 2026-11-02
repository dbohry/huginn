-- A subscribed feed. Ownership by a user is added with the accounts milestone.
CREATE TABLE feeds (
    id              INTEGER PRIMARY KEY,
    url             TEXT NOT NULL UNIQUE,
    title           TEXT NOT NULL DEFAULT '',
    site_url        TEXT NOT NULL DEFAULT '',
    etag            TEXT NOT NULL DEFAULT '',
    last_modified   TEXT NOT NULL DEFAULT '',
    last_fetched_at TEXT,
    last_error      TEXT NOT NULL DEFAULT '',
    error_count     INTEGER NOT NULL DEFAULT 0,
    next_fetch_at   TEXT,
    created_at      TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
) STRICT;

-- One article of a feed. (feed_id, guid) is unique so a refresh never duplicates.
CREATE TABLE items (
    id           INTEGER PRIMARY KEY,
    feed_id      INTEGER NOT NULL REFERENCES feeds(id) ON DELETE CASCADE,
    guid         TEXT NOT NULL,
    title        TEXT NOT NULL DEFAULT '',
    url          TEXT NOT NULL DEFAULT '',
    author       TEXT NOT NULL DEFAULT '',
    published_at TEXT,
    summary      TEXT NOT NULL DEFAULT '',
    content_html TEXT NOT NULL DEFAULT '',
    is_read      INTEGER NOT NULL DEFAULT 0 CHECK (is_read IN (0, 1)),
    created_at   TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (feed_id, guid)
) STRICT;

CREATE INDEX items_feed_published ON items (feed_id, published_at DESC);
CREATE INDEX items_unread ON items (is_read, published_at DESC);
