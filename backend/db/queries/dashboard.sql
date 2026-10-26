-- name: GetDashboardTotals :one
-- How much a student has: words, lessons, PDFs, highlights and notes.
SELECT
    (SELECT count(*) FROM vocabulary_entries v WHERE v.owner_id = @owner_id) AS words,
    (SELECT count(*) FROM lessons l WHERE l.owner_id = @owner_id) AS lessons,
    (SELECT count(*) FROM documents d WHERE d.owner_id = @owner_id) AS documents,
    (SELECT count(*) FROM annotations a WHERE a.owner_id = @owner_id) AS highlights,
    (SELECT count(*) FROM annotations a WHERE a.owner_id = @owner_id AND a.note <> '') AS notes;

-- name: ListActivityDays :many
-- What a student did on each day since @since, by the date in @time_zone:
-- words added, text highlighted, pages drawn or written on and flashcard
-- answers. Days with nothing done are left out.
WITH work AS (
    SELECT v.created_at AS at, 'word' AS kind, NULL::uuid AS document_id, NULL::integer AS page_number
    FROM vocabulary_entries v
    WHERE v.owner_id = @owner_id AND v.created_at >= @since
    UNION ALL
    SELECT a.created_at, 'highlight', a.document_id, a.page_number
    FROM annotations a
    WHERE a.owner_id = @owner_id AND a.created_at >= @since
    UNION ALL
    SELECT s.created_at, 'page', s.document_id, s.page_number
    FROM ink_strokes s
    WHERE s.owner_id = @owner_id AND s.created_at >= @since
    UNION ALL
    SELECT t.created_at, 'page', t.document_id, t.page_number
    FROM text_boxes t
    WHERE t.owner_id = @owner_id AND t.created_at >= @since
    UNION ALL
    SELECT r.reviewed_at, 'review', NULL, NULL
    FROM reviews r
    WHERE r.owner_id = @owner_id AND r.reviewed_at >= @since
)
SELECT to_char((at AT TIME ZONE @time_zone::text)::date, 'YYYY-MM-DD')::text AS day,
    count(*) FILTER (WHERE kind = 'word') AS words,
    count(*) FILTER (WHERE kind = 'highlight') AS highlights,
    count(DISTINCT (document_id, page_number)) FILTER (WHERE kind = 'page') AS pages,
    count(*) FILTER (WHERE kind = 'review') AS reviews
FROM work
GROUP BY 1
ORDER BY 1;

-- name: ListRecentVocabularyEntries :many
SELECT id, term, translation, created_at
FROM vocabulary_entries
WHERE owner_id = @owner_id
ORDER BY created_at DESC, id DESC
LIMIT @row_limit;

-- name: ListRecentDocuments :many
-- A student's documents by when they last worked on them (uploaded,
-- highlighted, drew or wrote on one), with the page of that last change.
WITH work AS (
    SELECT d.id AS document_id, 1::integer AS page_number, d.created_at AS at
    FROM documents d
    WHERE d.owner_id = @owner_id
    UNION ALL
    SELECT a.document_id, a.page_number, a.updated_at
    FROM annotations a
    WHERE a.owner_id = @owner_id
    UNION ALL
    SELECT s.document_id, s.page_number, s.created_at
    FROM ink_strokes s
    WHERE s.owner_id = @owner_id
    UNION ALL
    SELECT t.document_id, t.page_number, t.updated_at
    FROM text_boxes t
    WHERE t.owner_id = @owner_id
), latest AS (
    SELECT DISTINCT ON (document_id) document_id, page_number, at
    FROM work
    ORDER BY document_id, at DESC
)
SELECT d.id, d.title, d.lesson_id, l.title AS lesson_title, latest.page_number, latest.at AS worked_at
FROM latest
JOIN documents d ON d.id = latest.document_id
JOIN lessons l ON l.id = d.lesson_id
ORDER BY latest.at DESC, d.id
LIMIT @row_limit;
