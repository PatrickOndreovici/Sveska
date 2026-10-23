-- (id, owner_id) becomes a foreign key target, as lessons' already is, so
-- an annotation can only be added to a document its owner owns.
ALTER TABLE documents ADD CONSTRAINT documents_id_owner_key UNIQUE (id, owner_id);

-- A highlight on a PDF page, with an optional note. It is anchored by page
-- number and rectangles in 0–1 page coordinates, so it stays in place at
-- any zoom, and the selected text is kept as the quote.
CREATE TABLE annotations (
    id          uuid        PRIMARY KEY DEFAULT uuidv7(),
    owner_id    uuid        NOT NULL,
    document_id uuid        NOT NULL,
    page_number integer     NOT NULL,
    -- [{"x": 0.1, "y": 0.2, "width": 0.5, "height": 0.02}, ...], one per
    -- line of the selection. The service checks each value is within 0–1.
    rects       jsonb       NOT NULL,
    quote       text        NOT NULL,
    note        text        NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),

    -- The document's own foreign key reaches lessons and users.
    CONSTRAINT annotations_document_fkey FOREIGN KEY (document_id, owner_id)
        REFERENCES documents (id, owner_id) ON DELETE CASCADE,
    CONSTRAINT annotations_id_owner_key UNIQUE (id, owner_id),
    CONSTRAINT annotations_page_number_positive CHECK (page_number >= 1),
    CONSTRAINT annotations_rects_count CHECK (
        jsonb_typeof(rects) = 'array' AND jsonb_array_length(rects) BETWEEN 1 AND 200
    ),
    CONSTRAINT annotations_quote_length CHECK (char_length(quote) BETWEEN 1 AND 2000),
    CONSTRAINT annotations_note_length CHECK (char_length(note) <= 5000)
);

-- Serves a document's annotations in reading order and the cascade from documents.
CREATE INDEX annotations_document_page_idx ON annotations (document_id, page_number, created_at);
