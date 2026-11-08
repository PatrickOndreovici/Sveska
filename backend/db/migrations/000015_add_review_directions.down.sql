-- English → Serbian cards go; their answers stay, as they still count as
-- activity on the dashboard.
ALTER TABLE reviews DROP COLUMN IF EXISTS direction;
DELETE FROM review_cards WHERE direction = 'en_sr';
ALTER TABLE review_cards DROP CONSTRAINT review_cards_pkey;
ALTER TABLE review_cards ADD PRIMARY KEY (entry_id);
ALTER TABLE review_cards DROP COLUMN IF EXISTS direction;
DROP TYPE IF EXISTS review_direction;
