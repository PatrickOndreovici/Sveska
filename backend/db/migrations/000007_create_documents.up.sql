-- Metadata for an uploaded PDF. The bytes live in file storage under
-- storage_key; the database never holds them.
CREATE TABLE documents (
    id          uuid        PRIMARY KEY DEFAULT uuidv7(),
    owner_id    uuid        NOT NULL,
    lesson_id   uuid        NOT NULL,
    title       text        NOT NULL,
    storage_key text        NOT NULL,
    byte_size   bigint      NOT NULL,
    -- SHA-256 of the bytes, to detect corruption and duplicates later.
    sha256      bytea       NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),

    -- Pairing lesson_id with owner_id means a document can only be added to
    -- a lesson its owner owns; the lesson's own foreign key reaches users.
    CONSTRAINT documents_lesson_fkey FOREIGN KEY (lesson_id, owner_id)
        REFERENCES lessons (id, owner_id) ON DELETE CASCADE,
    CONSTRAINT documents_storage_key_key UNIQUE (storage_key),
    CONSTRAINT documents_title_length CHECK (char_length(title) BETWEEN 1 AND 200),
    CONSTRAINT documents_byte_size_positive CHECK (byte_size > 0),
    CONSTRAINT documents_sha256_length CHECK (octet_length(sha256) = 32)
);

-- Serves a lesson's list in upload order and the cascade from lessons.
CREATE INDEX documents_lesson_created_idx ON documents (lesson_id, created_at);
