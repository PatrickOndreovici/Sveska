-- Every query filters on owner_id: someone else's text box behaves exactly
-- like a missing one.

-- name: CreateTextBox :one
-- Fails on text_boxes_document_fkey when the document does not exist or
-- belongs to someone else.
INSERT INTO text_boxes (owner_id, document_id, page_number, x, y, body, color, font_size)
VALUES (@owner_id, @document_id, @page_number, @x, @y, @body, @color, @font_size)
RETURNING *;

-- name: ListDocumentTextBoxes :many
-- A document's text boxes page by page, in the order they were added.
SELECT *
FROM text_boxes
WHERE document_id = @document_id
  AND owner_id = @owner_id
ORDER BY page_number, id;

-- name: UpdateTextBox :one
-- Its text, colour, size and place on the page change; the page does not.
UPDATE text_boxes
SET x          = @x,
    y          = @y,
    body       = @body,
    color      = @color,
    font_size  = @font_size,
    updated_at = now()
WHERE id = @id
  AND owner_id = @owner_id
RETURNING *;

-- name: DeleteTextBox :execrows
-- Returns the number of deleted rows; 0 means not found (or not yours).
DELETE FROM text_boxes
WHERE id = @id
  AND owner_id = @owner_id;
