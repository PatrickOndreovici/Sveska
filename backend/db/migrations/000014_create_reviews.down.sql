DROP TABLE IF EXISTS reviews;
DROP TABLE IF EXISTS review_cards;
ALTER TABLE vocabulary_entries DROP CONSTRAINT IF EXISTS vocabulary_entries_id_owner_key;
DROP TYPE IF EXISTS review_rating;
