CREATE TABLE vocabulary_entries (
    id          uuid        PRIMARY KEY DEFAULT uuidv7(),
    owner_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    term        text        NOT NULL,
    translation text        NOT NULL,
    -- Empty rather than NULL when there are no notes, so Go reads a plain string.
    notes       text        NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT vocabulary_entries_term_length CHECK (char_length(term) BETWEEN 1 AND 200),
    CONSTRAINT vocabulary_entries_translation_length CHECK (char_length(translation) BETWEEN 1 AND 200),
    CONSTRAINT vocabulary_entries_notes_length CHECK (char_length(notes) <= 2000)
);

-- The same term may have several translations, but each pair only once per
-- owner, ignoring the case of the term. An expression needs a unique index,
-- not a UNIQUE constraint. Its leading owner_id column also serves the
-- foreign key and per-owner lookups, so no separate owner_id index is needed.
CREATE UNIQUE INDEX vocabulary_entries_owner_term_translation_key
    ON vocabulary_entries (owner_id, lower(term), translation);
