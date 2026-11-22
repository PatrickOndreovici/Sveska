-- Every query filters on owner_id: someone else's stroke behaves exactly
-- like a missing one.

-- name: CreateInkStroke :one
-- Fails on ink_strokes_document_fkey when the document does not exist or
-- belongs to someone else.
INSERT INTO ink_strokes (owner_id, document_id, page_number, color, points)
VALUES (@owner_id, @document_id, @page_number, @color, @points)
RETURNING *;

-- name: ListDocumentInkStrokes :many
-- A document's strokes page by page, in the order they were drawn.
SELECT *
FROM ink_strokes
WHERE document_id = @document_id
  AND owner_id = @owner_id
ORDER BY page_number, id;

-- name: DeleteInkStroke :execrows
-- Returns the number of deleted rows; 0 means not found (or not yours).
DELETE FROM ink_strokes
WHERE id = @id
  AND owner_id = @owner_id;
