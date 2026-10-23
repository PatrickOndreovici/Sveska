-- Text typed onto a PDF page, such as the answer in a fill-in exercise.
-- (x, y) is its top-left corner in 0–1 page coordinates; the text wraps at
-- the page's right edge.
CREATE TABLE text_boxes (
    id          uuid        PRIMARY KEY DEFAULT uuidv7(),
    owner_id    uuid        NOT NULL,
    document_id uuid        NOT NULL,
    page_number integer     NOT NULL,
    x           float8      NOT NULL,
    y           float8      NOT NULL,
    body        text        NOT NULL,
    color       ink_color   NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT text_boxes_document_fkey FOREIGN KEY (document_id, owner_id)
        REFERENCES documents (id, owner_id) ON DELETE CASCADE,
    CONSTRAINT text_boxes_page_number_positive CHECK (page_number >= 1),
    -- Written so that NaN, which PostgreSQL sorts above every number, fails too.
    CONSTRAINT text_boxes_position_on_page CHECK (x >= 0 AND x < 1 AND y >= 0 AND y < 1),
    CONSTRAINT text_boxes_body_length CHECK (char_length(body) BETWEEN 1 AND 1000)
);

-- Serves a document's text boxes page by page and the cascade from documents.
CREATE INDEX text_boxes_document_page_idx ON text_boxes (document_id, page_number, id);
