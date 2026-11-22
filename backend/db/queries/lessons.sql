-- Every query filters on owner_id, like vocabulary: someone else's lesson
-- behaves exactly like a missing one.

-- name: CreateLesson :one
INSERT INTO lessons (owner_id, title, description)
VALUES (@owner_id, @title, @description)
RETURNING *;

-- name: ListLessons :many
-- The owner's lessons, newest first, each with how many PDFs it holds.
-- A student keeps tens of lessons, not thousands, so there is no paging.
SELECT
    l.id, l.owner_id, l.title, l.description, l.created_at, l.updated_at,
    (SELECT count(*) FROM documents d WHERE d.lesson_id = l.id)::int AS document_count
FROM lessons l
WHERE l.owner_id = @owner_id
ORDER BY l.created_at DESC, l.id DESC;

-- name: GetLesson :one
SELECT *
FROM lessons
WHERE id = @id
  AND owner_id = @owner_id;

-- name: UpdateLesson :one
UPDATE lessons
SET title       = @title,
    description = @description,
    updated_at  = now()
WHERE id = @id
  AND owner_id = @owner_id
RETURNING *;

-- name: DeleteLesson :one
-- Deletes the lesson; its documents' rows go with it through the foreign
-- key. Returns those documents' storage keys so the caller can delete the
-- files as well. The SELECT reads the snapshot from before the DELETE, so it
-- still sees the documents. No row at all means not found (or not yours).
WITH deleted AS (
    DELETE FROM lessons l
    WHERE l.id = @id
      AND l.owner_id = @owner_id
    RETURNING l.id
)
SELECT coalesce(array_agg(d.storage_key) FILTER (WHERE d.storage_key IS NOT NULL), '{}')::text[] AS storage_keys
FROM deleted
LEFT JOIN documents d ON d.lesson_id = deleted.id
GROUP BY deleted.id;
