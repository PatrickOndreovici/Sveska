-- Flashcards. Every query filters on owner_id, as vocabulary's do, so
-- someone else's word behaves exactly like a missing one.

-- name: ListDueReviewCards :many
-- The words due on or before @today with their schedules, longest overdue
-- first.
SELECT sqlc.embed(v), c.due_on, c.interval_days, c.ease_percent, c.repetitions, c.lapses, c.last_reviewed_at
FROM review_cards c
JOIN vocabulary_entries v ON v.id = c.entry_id
WHERE c.owner_id = @owner_id
  AND c.due_on <= @today
ORDER BY c.due_on, c.last_reviewed_at, c.entry_id
LIMIT @row_limit;

-- name: ListNewReviewEntries :many
-- Words never reviewed, oldest first, so none waits for ever.
SELECT v.*
FROM vocabulary_entries v
WHERE v.owner_id = @owner_id
  AND NOT EXISTS (SELECT 1 FROM review_cards c WHERE c.entry_id = v.id)
ORDER BY v.created_at, v.id
LIMIT @row_limit;

-- name: GetReviewCounts :one
-- due: cards due on or before @today. unseen: words never reviewed.
-- started_today: words first reviewed since @day_start, which count
-- against the day's limit of new words.
SELECT
    (SELECT count(*) FROM review_cards c WHERE c.owner_id = @owner_id AND c.due_on <= @today) AS due,
    (SELECT count(*) FROM vocabulary_entries v
     WHERE v.owner_id = @owner_id
       AND NOT EXISTS (SELECT 1 FROM review_cards n WHERE n.entry_id = v.id)) AS unseen,
    (SELECT count(*) FROM review_cards s WHERE s.owner_id = @owner_id AND s.created_at >= @day_start) AS started_today;

-- name: GetNextReviewDate :one
-- The first day after @today with a card due; no row when there is none.
SELECT due_on
FROM review_cards
WHERE owner_id = @owner_id
  AND due_on > @today
ORDER BY due_on
LIMIT 1;

-- name: GetReviewCard :one
SELECT *
FROM review_cards
WHERE entry_id = @entry_id
  AND owner_id = @owner_id;

-- name: RecordReview :one
-- Saves a word's new schedule and logs the answer in one statement, so
-- both happen or neither. The schedule is replaced only if it is still the
-- one the service read (@read_reviewed_at, NULL for a new word); if another
-- answer got there first, nothing changes and no row comes back.
WITH card AS (
    INSERT INTO review_cards (
        entry_id, owner_id, due_on, interval_days, ease_percent, repetitions, lapses, last_reviewed_at
    )
    VALUES (@entry_id, @owner_id, @due_on, @interval_days, @ease_percent, @repetitions, @lapses, now())
    ON CONFLICT (entry_id) DO UPDATE
    SET due_on           = EXCLUDED.due_on,
        interval_days    = EXCLUDED.interval_days,
        ease_percent     = EXCLUDED.ease_percent,
        repetitions      = EXCLUDED.repetitions,
        lapses           = EXCLUDED.lapses,
        last_reviewed_at = EXCLUDED.last_reviewed_at
    WHERE review_cards.owner_id = EXCLUDED.owner_id
      AND review_cards.last_reviewed_at IS NOT DISTINCT FROM sqlc.narg(read_reviewed_at)::timestamptz
    RETURNING *
), logged AS (
    INSERT INTO reviews (owner_id, entry_id, rating, interval_days)
    SELECT card.owner_id, card.entry_id, @rating::review_rating, card.interval_days
    FROM card
)
SELECT *
FROM card;
