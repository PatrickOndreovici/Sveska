-- A user's profile picture. The API re-encodes every upload as a small
-- JPEG of at most 256×256, so the bytes stay a few dozen kilobytes and are
-- kept here rather than in file storage: they go with the user, and
-- there is no file to clean up.
CREATE TABLE user_avatars (
    user_id    uuid        PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    image      bytea       NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT user_avatars_image_size CHECK (octet_length(image) BETWEEN 1 AND 1048576)
);
