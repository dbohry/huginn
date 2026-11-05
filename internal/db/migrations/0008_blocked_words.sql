-- Words a user does not want to see. An article whose headline contains one of
-- them as a whole word is hidden from that user. Words are stored lower case.
CREATE TABLE blocked_words (
    id      INTEGER PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    word    TEXT NOT NULL,
    UNIQUE (user_id, word)
) STRICT;
