-- Every query filters on owner_id: someone else's annotation behaves
-- exactly like a missing one.

-- name: CreateAnnotation :one
-- Fails on annotations_document_fkey when the document does not exist or
-- belongs to someone else.
INSERT INTO annotations (owner_id, document_id, page_number, rects, quote, note)
VALUES (@owner_id, @document_id, @page_number, @rects, @quote, @note)
RETURNING *;

-- name: ListDocumentAnnotations :many
-- A document's annotations in reading order.
SELECT *
FROM annotations
WHERE document_id = @document_id
  AND owner_id = @owner_id
ORDER BY page_number, created_at, id;

-- name: UpdateAnnotationNote :one
-- The note is the only thing that changes; the highlighted text does not.
UPDATE annotations
SET note       = @note,
    updated_at = now()
WHERE id = @id
  AND owner_id = @owner_id
RETURNING *;

-- name: DeleteAnnotation :execrows
-- Returns the number of deleted rows; 0 means not found (or not yours).
-- Words saved from it keep existing; only their link is cleared.
DELETE FROM annotations
WHERE id = @id
  AND owner_id = @owner_id;

-- name: GetAnnotationSource :one
-- Where an annotation is: its document and page, for a word saved from it.
SELECT a.id, a.document_id, a.page_number, d.title AS document_title
FROM annotations a
JOIN documents d ON d.id = a.document_id
WHERE a.id = @id
  AND a.owner_id = @owner_id;

-- name: ListEntrySources :many
-- The highlights that the given words were saved from, for showing
-- "from Čitanka, page 3" next to them. Words without one are left out.
SELECT v.id AS entry_id, a.id AS annotation_id, a.document_id, a.page_number, d.title AS document_title
FROM vocabulary_entries v
JOIN annotations a ON a.id = v.source_annotation_id
JOIN documents d ON d.id = a.document_id
WHERE v.owner_id = @owner_id
  AND v.id = ANY (@entry_ids::uuid[]);
