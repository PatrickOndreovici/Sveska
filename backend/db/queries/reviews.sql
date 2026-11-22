-- Flashcards. Every query filters on owner_id, as vocabulary's do, so
-- someone else's word behaves exactly like a missing one.
--
-- A word has a Serbian-to-English card (sr_en) from its first review and an
-- English-to-Serbian one (en_sr) once the first is learned. A word shows one
-- of them a day, so that one card does not give away the answer to the
-- other: a card waits for tomorrow while the other card of its word was
-- reviewed today.

-- name: ListDueReviewCards :many
-- The cards due on or before @today with their words, longest overdue
-- first, at most one per word. A card is left out while the other card of
-- its word was reviewed today, or is due as well and comes first in this order;
-- once that one is answered, this one waits for tomorrow. (A card reviewed
-- today and due again does not give way: it was the card of the day.)
SELECT sqlc.embed(v), c.direction, c.due_on, c.interval_days, c.ease_percent, c.repetitions, c.lapses, c.last_reviewed_at
FROM review_cards c
JOIN vocabulary_entries v ON v.id = c.entry_id
WHERE c.owner_id = @owner_id
  AND c.due_on <= @today
  AND NOT EXISTS (
      SELECT 1 FROM review_cards o
      WHERE o.entry_id = c.entry_id
        AND o.direction <> c.direction
        AND (o.last_reviewed_at >= @day_start
             OR (o.due_on <= @today
                 AND c.last_reviewed_at < @day_start
                 AND (o.due_on, o.last_reviewed_at, o.direction) < (c.due_on, c.last_reviewed_at, c.direction)))
  )
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

-- name: ListNewRecallEntries :many
-- Words ready for their first English-to-Serbian card, those learned first
-- first: their Serbian-to-English card was remembered @min_repetitions times
-- in a row and is neither due nor reviewed today.
SELECT sqlc.embed(v)
FROM review_cards c
JOIN vocabulary_entries v ON v.id = c.entry_id
WHERE c.owner_id = @owner_id
  AND c.direction = 'sr_en'
  AND c.repetitions >= @min_repetitions
  AND c.due_on > @today
  AND c.last_reviewed_at < @day_start
  AND NOT EXISTS (SELECT 1 FROM review_cards r WHERE r.entry_id = c.entry_id AND r.direction = 'en_sr')
ORDER BY c.created_at, c.entry_id
LIMIT @row_limit;

-- name: GetReviewCounts :one
-- due: words with a card due on or before @today that can show today.
-- unseen: words never reviewed. started_today: words first reviewed since
-- @day_start, which count against the day's limit of new words.
-- recall_waiting: words remembered well enough for an English-to-Serbian
-- card that have none yet; recall_ready: those whose card can start today
-- (see ListNewRecallEntries). recall_started_today: English-to-Serbian
-- cards started since @day_start, which have a daily limit of their own.
WITH recall AS (
    SELECT
        count(*) AS waiting,
        count(*) FILTER (WHERE c.due_on > @today AND c.last_reviewed_at < @day_start) AS ready
    FROM review_cards c
    WHERE c.owner_id = @owner_id
      AND c.direction = 'sr_en'
      AND c.repetitions >= @min_repetitions
      AND NOT EXISTS (SELECT 1 FROM review_cards r WHERE r.entry_id = c.entry_id AND r.direction = 'en_sr')
)
SELECT
    (SELECT count(DISTINCT d.entry_id) FROM review_cards d
     WHERE d.owner_id = @owner_id
       AND d.due_on <= @today
       AND NOT EXISTS (
           SELECT 1 FROM review_cards o
           WHERE o.entry_id = d.entry_id
             AND o.direction <> d.direction
             AND o.last_reviewed_at >= @day_start
       )) AS due,
    (SELECT count(*) FROM vocabulary_entries v
     WHERE v.owner_id = @owner_id
       AND NOT EXISTS (SELECT 1 FROM review_cards n WHERE n.entry_id = v.id)) AS unseen,
    (SELECT count(*) FROM review_cards s
     WHERE s.owner_id = @owner_id AND s.direction = 'sr_en' AND s.created_at >= @day_start) AS started_today,
    recall.waiting AS recall_waiting,
    recall.ready AS recall_ready,
    (SELECT count(*) FROM review_cards s
     WHERE s.owner_id = @owner_id AND s.direction = 'en_sr' AND s.created_at >= @day_start) AS recall_started_today
FROM recall;

-- name: GetNextReviewDate :one
-- The first day from @tomorrow on with a card due; no row when there is
-- none. A card due by today that waits because the other card of its word
-- was reviewed today comes tomorrow.
SELECT GREATEST(c.due_on, @tomorrow::date)::date AS due_on
FROM review_cards c
WHERE c.owner_id = @owner_id
  AND (c.due_on >= @tomorrow OR EXISTS (
      SELECT 1 FROM review_cards o
      WHERE o.entry_id = c.entry_id
        AND o.direction <> c.direction
        AND o.last_reviewed_at >= @day_start
  ))
ORDER BY 1
LIMIT 1;

-- name: GetReviewCard :one
SELECT *
FROM review_cards
WHERE entry_id = @entry_id
  AND direction = @direction
  AND owner_id = @owner_id;

-- name: RecordReview :one
-- Saves a card's new schedule and logs the answer in one statement, so
-- both happen or neither. The schedule is replaced only if it is still the
-- one the service read (@read_reviewed_at, NULL for a new card); if another
-- answer got there first, nothing changes and no row comes back.
WITH card AS (
    INSERT INTO review_cards (
        entry_id, direction, owner_id, due_on, interval_days, ease_percent, repetitions, lapses, last_reviewed_at
    )
    VALUES (@entry_id, @direction, @owner_id, @due_on, @interval_days, @ease_percent, @repetitions, @lapses, now())
    ON CONFLICT (entry_id, direction) DO UPDATE
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
    INSERT INTO reviews (owner_id, entry_id, direction, rating, interval_days)
    SELECT card.owner_id, card.entry_id, card.direction, @rating::review_rating, card.interval_days
    FROM card
)
SELECT *
FROM card;
