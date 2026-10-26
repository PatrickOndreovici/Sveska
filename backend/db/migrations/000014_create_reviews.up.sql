-- Flashcards: each of a student's words is a card, shown Serbian side first,
-- and comes back on a schedule that stretches each time it is remembered
-- (spaced repetition). review_cards keeps a word's schedule from its first
-- review on, so a word without a row is new. reviews logs every answer.
CREATE TYPE review_rating AS ENUM ('again', 'hard', 'good', 'easy');

-- (id, owner_id) becomes a foreign key target, as documents' already is, so
-- a card or a review can only be about its owner's own word.
ALTER TABLE vocabulary_entries ADD CONSTRAINT vocabulary_entries_id_owner_key UNIQUE (id, owner_id);

CREATE TABLE review_cards (
    entry_id         uuid        PRIMARY KEY,
    owner_id         uuid        NOT NULL,
    -- The date the word is shown next, in the student's time zone.
    due_on           date        NOT NULL,
    -- Days from the last review to due_on: 0 for a word forgotten today,
    -- at most a year so that every word comes back now and then.
    interval_days    integer     NOT NULL,
    -- How much a "good" answer stretches the interval: 250 multiplies it by 2.5.
    ease_percent     integer     NOT NULL,
    -- Answers in a row other than "again"; back to 0 when the word is forgotten.
    repetitions      integer     NOT NULL,
    -- How many times the word was forgotten.
    lapses           integer     NOT NULL,
    last_reviewed_at timestamptz NOT NULL,
    -- When the word was first reviewed.
    created_at       timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT review_cards_entry_fkey FOREIGN KEY (entry_id, owner_id)
        REFERENCES vocabulary_entries (id, owner_id) ON DELETE CASCADE,
    CONSTRAINT review_cards_interval_range CHECK (interval_days BETWEEN 0 AND 365),
    CONSTRAINT review_cards_ease_min CHECK (ease_percent >= 130),
    CONSTRAINT review_cards_counts_positive CHECK (repetitions >= 0 AND lapses >= 0)
);

-- Finds a student's due cards. The primary key serves the foreign key.
CREATE INDEX review_cards_owner_due_idx ON review_cards (owner_id, due_on);

CREATE TABLE reviews (
    id            uuid          PRIMARY KEY DEFAULT uuidv7(),
    owner_id      uuid          NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    -- NULL once the word is deleted: the review still counts for that day.
    entry_id      uuid,
    rating        review_rating NOT NULL,
    -- The interval the answer set.
    interval_days integer       NOT NULL,
    reviewed_at   timestamptz   NOT NULL DEFAULT now(),

    -- As for a word's source highlight, the column list keeps owner_id.
    CONSTRAINT reviews_entry_fkey FOREIGN KEY (entry_id, owner_id)
        REFERENCES vocabulary_entries (id, owner_id) ON DELETE SET NULL (entry_id),
    CONSTRAINT reviews_interval_range CHECK (interval_days BETWEEN 0 AND 365)
);

-- Counts a student's reviews by day for the dashboard.
CREATE INDEX reviews_owner_reviewed_idx ON reviews (owner_id, reviewed_at);
-- Deleting a word clears its reviews' entry_id.
CREATE INDEX reviews_entry_idx ON reviews (entry_id) WHERE entry_id IS NOT NULL;
