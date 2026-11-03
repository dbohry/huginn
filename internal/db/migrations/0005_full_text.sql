-- The full text of an article, loaded on request from the article's own page and
-- kept here so it is fetched only once.
ALTER TABLE items ADD COLUMN full_html TEXT NOT NULL DEFAULT '';

-- Whether to load the full article automatically when one of this feed's articles is opened.
ALTER TABLE feeds ADD COLUMN full_text INTEGER NOT NULL DEFAULT 0 CHECK (full_text IN (0, 1));
