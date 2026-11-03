-- Unread counts per feed read only the unread rows instead of every article.
CREATE INDEX items_feed_unread ON items (feed_id) WHERE is_read = 0;

-- Lets the "all articles" list walk articles newest first and stop after one page.
CREATE INDEX items_published ON items (published_at DESC, id DESC);
