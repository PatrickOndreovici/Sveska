-- name: SaveAvatar :one
-- Adds the user's picture or replaces it. updated_at versions the picture's URL.
INSERT INTO user_avatars (user_id, image)
VALUES (@user_id, @image)
ON CONFLICT (user_id) DO UPDATE
SET image      = EXCLUDED.image,
    updated_at = now()
RETURNING updated_at;

-- name: GetAvatar :one
SELECT image, updated_at
FROM user_avatars
WHERE user_id = @user_id;

-- name: DeleteAvatar :exec
DELETE FROM user_avatars
WHERE user_id = @user_id;
