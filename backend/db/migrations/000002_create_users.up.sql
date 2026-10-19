CREATE TYPE user_role AS ENUM ('student', 'teacher');

CREATE TABLE users (
    -- uuidv7 is time-ordered (PostgreSQL 18+), so inserts stay index-friendly
    -- while IDs remain safe to expose in URLs.
    id            uuid        PRIMARY KEY DEFAULT uuidv7(),
    email         citext      NOT NULL,
    password_hash text        NOT NULL,
    display_name  text        NOT NULL,
    role          user_role   NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT users_email_key UNIQUE (email),
    CONSTRAINT users_email_length CHECK (char_length(email) BETWEEN 3 AND 254),
    CONSTRAINT users_display_name_length CHECK (char_length(display_name) BETWEEN 1 AND 80)
);
