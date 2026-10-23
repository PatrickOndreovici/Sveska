-- Every query filters on owner_id: someone else's document behaves exactly
-- like a missing one.

-- name: CreateDocument :one
-- Fails on documents_lesson_fkey when the lesson does not exist or belongs
-- to someone else.
INSERT INTO documents (owner_id, lesson_id, title, storage_key, byte_size, sha256)
VALUES (@owner_id, @lesson_id, @title, @storage_key, @byte_size, @sha256)
RETURNING *;

-- name: ListLessonDocuments :many
-- A lesson's PDFs in upload order.
SELECT *
FROM documents
WHERE lesson_id = @lesson_id
  AND owner_id = @owner_id
ORDER BY created_at, id;

-- name: GetDocument :one
-- One document plus its lesson's title, for the viewer's way back.
SELECT d.id, d.owner_id, d.lesson_id, d.title, d.storage_key, d.byte_size, d.sha256, d.created_at,
       l.title AS lesson_title
FROM documents d
JOIN lessons l ON l.id = d.lesson_id
WHERE d.id = @id
  AND d.owner_id = @owner_id;

-- name: DeleteDocument :one
-- Returns the storage key so the caller can delete the file. No row means
-- not found (or not yours).
DELETE FROM documents
WHERE id = @id
  AND owner_id = @owner_id
RETURNING storage_key;
