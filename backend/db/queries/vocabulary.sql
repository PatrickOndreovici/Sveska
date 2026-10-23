-- Every query filters on owner_id: a user only ever reaches their own
-- entries, and someone else's id behaves exactly like a missing one.
-- Rows come back as "*", which sqlc expands at generate time into every
-- column, so it reuses the store.VocabularyEntry model and a new column
-- shows up in Go after the next `make sqlc`.

-- name: CreateVocabularyEntry :one
INSERT INTO vocabulary_entries (
    owner_id, term, translation, notes, example, part_of_speech,
    gender, plural, aspect, aspect_pair, feminine_form, neuter_form
)
VALUES (
    @owner_id, @term, @translation, @notes, @example, @part_of_speech,
    @gender, @plural, @aspect, @aspect_pair, @feminine_form, @neuter_form
)
RETURNING *;

-- name: ListVocabularyEntries :many
-- One page of entries.
--   query: '' for no search. Otherwise a case- and diacritic-insensitive
--          substring of the word, its translation or its other forms.
--          strpos rather than LIKE, so % and _ are matched literally.
--   days:  0 for all time, otherwise only entries added in the last N days.
--   sort:  'az' for Serbian alphabetical order, anything else for newest
--          first. Both end on created_at and id, so the order is total and
--          pages never overlap or skip a row.
-- Keep the WHERE clause in step with CountVocabularyEntries.
SELECT *
FROM vocabulary_entries
WHERE owner_id = @owner_id
  AND (
    @query::text = ''
    OR strpos(lower(unaccent(term)), lower(unaccent(@query::text))) > 0
    OR strpos(lower(unaccent(translation)), lower(unaccent(@query::text))) > 0
    OR strpos(lower(unaccent(plural)), lower(unaccent(@query::text))) > 0
    OR strpos(lower(unaccent(aspect_pair)), lower(unaccent(@query::text))) > 0
    OR strpos(lower(unaccent(feminine_form)), lower(unaccent(@query::text))) > 0
    OR strpos(lower(unaccent(neuter_form)), lower(unaccent(@query::text))) > 0
  )
  AND (@days::int = 0 OR created_at >= now() - make_interval(days => @days::int))
ORDER BY
  -- For 'newest' both CASEs are NULL for every row, so they do not affect the order.
  CASE WHEN @sort::text = 'az' THEN term COLLATE "sr-Latn-x-icu" END,
  CASE WHEN @sort::text = 'az' THEN translation COLLATE "sr-Latn-x-icu" END,
  created_at DESC,
  id DESC
LIMIT @page_limit::int
OFFSET @page_offset::int;

-- name: CountVocabularyEntries :one
-- The total behind ListVocabularyEntries ("11–20 of 44"), same filters.
SELECT count(*)
FROM vocabulary_entries
WHERE owner_id = @owner_id
  AND (
    @query::text = ''
    OR strpos(lower(unaccent(term)), lower(unaccent(@query::text))) > 0
    OR strpos(lower(unaccent(translation)), lower(unaccent(@query::text))) > 0
    OR strpos(lower(unaccent(plural)), lower(unaccent(@query::text))) > 0
    OR strpos(lower(unaccent(aspect_pair)), lower(unaccent(@query::text))) > 0
    OR strpos(lower(unaccent(feminine_form)), lower(unaccent(@query::text))) > 0
    OR strpos(lower(unaccent(neuter_form)), lower(unaccent(@query::text))) > 0
  )
  AND (@days::int = 0 OR created_at >= now() - make_interval(days => @days::int));

-- name: GetVocabularyEntry :one
SELECT *
FROM vocabulary_entries
WHERE id = @id
  AND owner_id = @owner_id;

-- name: UpdateVocabularyEntry :one
-- Replaces all editable fields; the edit form always sends the full set.
UPDATE vocabulary_entries
SET term           = @term,
    translation    = @translation,
    notes          = @notes,
    example        = @example,
    part_of_speech = @part_of_speech,
    gender         = @gender,
    plural         = @plural,
    aspect         = @aspect,
    aspect_pair    = @aspect_pair,
    feminine_form  = @feminine_form,
    neuter_form    = @neuter_form,
    updated_at     = now()
WHERE id = @id
  AND owner_id = @owner_id
RETURNING *;

-- name: DeleteVocabularyEntry :execrows
-- Returns the number of deleted rows; 0 means not found (or not yours).
DELETE FROM vocabulary_entries
WHERE id = @id
  AND owner_id = @owner_id;
