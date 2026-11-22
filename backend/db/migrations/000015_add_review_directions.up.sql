-- Flashcards both ways: besides Serbian → English, a word gets a card shown
-- English side first and answered by typing the Serbian word. Each way has
-- its own schedule, so a word now has up to two cards.
CREATE TYPE review_direction AS ENUM ('sr_en', 'en_sr');

-- Every card and answer so far was Serbian first. The default only fills
-- the existing rows: new ones always say which way they go.
ALTER TABLE review_cards ADD COLUMN direction review_direction NOT NULL DEFAULT 'sr_en';
ALTER TABLE review_cards ALTER COLUMN direction DROP DEFAULT;
ALTER TABLE review_cards DROP CONSTRAINT review_cards_pkey;
-- Still leads with entry_id, so it serves the foreign key as before.
ALTER TABLE review_cards ADD PRIMARY KEY (entry_id, direction);

ALTER TABLE reviews ADD COLUMN direction review_direction NOT NULL DEFAULT 'sr_en';
ALTER TABLE reviews ALTER COLUMN direction DROP DEFAULT;
