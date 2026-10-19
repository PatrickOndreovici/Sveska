-- Case-insensitive text, used for unique email addresses on the users table
-- (see docs/data-model.md). gen_random_uuid() is built into PostgreSQL 13+.
CREATE EXTENSION IF NOT EXISTS citext;
