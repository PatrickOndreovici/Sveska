-- Colours for writing and drawing on a page. Shared by ink strokes and
-- text boxes.
CREATE TYPE ink_color AS ENUM ('black', 'blue', 'red');

-- A line drawn by hand on a PDF page, such as a circle around an answer:
-- the points of one movement of the pen, from touching the page to lifting
-- it. Like highlights they are in 0–1 page coordinates, so the drawing
-- fits the page at any zoom. A stroke never changes; it is drawn or erased.
CREATE TABLE ink_strokes (
    id          uuid        PRIMARY KEY DEFAULT uuidv7(),
    owner_id    uuid        NOT NULL,
    document_id uuid        NOT NULL,
    page_number integer     NOT NULL,
    color       ink_color   NOT NULL,
    -- [{"x": 0.1, "y": 0.2}, ...] in drawing order. The service checks
    -- each value is within 0–1.
    points      jsonb       NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT ink_strokes_document_fkey FOREIGN KEY (document_id, owner_id)
        REFERENCES documents (id, owner_id) ON DELETE CASCADE,
    CONSTRAINT ink_strokes_page_number_positive CHECK (page_number >= 1),
    CONSTRAINT ink_strokes_points_count CHECK (
        jsonb_typeof(points) = 'array' AND jsonb_array_length(points) BETWEEN 1 AND 1000
    )
);

-- Serves a document's strokes page by page in drawing order (ids are
-- UUIDv7, so they sort by creation) and the cascade from documents.
CREATE INDEX ink_strokes_document_page_idx ON ink_strokes (document_id, page_number, id);
