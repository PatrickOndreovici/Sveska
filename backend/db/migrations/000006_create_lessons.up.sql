CREATE TABLE lessons (
    id          uuid        PRIMARY KEY DEFAULT uuidv7(),
    owner_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    title       text        NOT NULL,
    -- Empty rather than NULL when there is no description, so Go reads a plain string.
    description text        NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),

    -- Target of the composite foreign key from documents, which keeps a
    -- document's owner equal to its lesson's owner.
    CONSTRAINT lessons_id_owner_key UNIQUE (id, owner_id),
    CONSTRAINT lessons_title_length CHECK (char_length(title) BETWEEN 1 AND 200),
    CONSTRAINT lessons_description_length CHECK (char_length(description) <= 2000)
);

-- Serves the owner's list, newest first, and the foreign key to users.
CREATE INDEX lessons_owner_created_idx ON lessons (owner_id, created_at DESC);
