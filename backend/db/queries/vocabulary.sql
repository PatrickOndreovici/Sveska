-- Every query filters on owner_id: a user only ever reaches their own
-- entries, and someone else's id behaves exactly like a missing one.
-- Each returns every column, so sqlc reuses the store.VocabularyEntry model.

-- name: CreateVocabularyEntry :one
INSERT INTO vocabulary_entries (owner_id, term, translation, notes)
VALUES (@owner_id, @term, @translation, @notes)
RETURNING id, owner_id, term, translation, notes, created_at, updated_at;

-- name: ListVocabularyEntries :many
-- Newest first. An empty query returns every entry. strpos rather than
-- LIKE, so % and _ typed by the user are matched literally.
SELECT id, owner_id, term, translation, notes, created_at, updated_at
FROM vocabulary_entries
WHERE owner_id = @owner_id
  AND (
    @query::text = ''
    OR strpos(lower(term), lower(@query::text)) > 0
    OR strpos(lower(translation), lower(@query::text)) > 0
  )
ORDER BY created_at DESC, id DESC;

-- name: GetVocabularyEntry :one
SELECT id, owner_id, term, translation, notes, created_at, updated_at
FROM vocabulary_entries
WHERE id = @id
  AND owner_id = @owner_id;

-- name: UpdateVocabularyEntry :one
-- Replaces all editable fields; the service merges a partial update first.
UPDATE vocabulary_entries
SET term        = @term,
    translation = @translation,
    notes       = @notes,
    updated_at  = now()
WHERE id = @id
  AND owner_id = @owner_id
RETURNING id, owner_id, term, translation, notes, created_at, updated_at;

-- name: DeleteVocabularyEntry :execrows
-- Returns the number of deleted rows; 0 means not found (or not yours).
DELETE FROM vocabulary_entries
WHERE id = @id
  AND owner_id = @owner_id;
