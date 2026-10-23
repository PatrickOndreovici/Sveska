-- name: CreateUser :one
INSERT INTO users (email, password_hash, display_name, role)
VALUES (@email, @password_hash, @display_name, @role)
RETURNING id, email, display_name, role, created_at;

-- name: GetUserCredentialsByEmail :one
SELECT u.id, u.email, u.password_hash, u.display_name, u.role, u.created_at, a.updated_at AS avatar_updated_at
FROM users u
LEFT JOIN user_avatars a ON a.user_id = u.id
WHERE u.email = @email;

-- name: UpdateDisplayName :one
-- Returns the user as GetSessionUser does, picture included.
WITH updated AS (
    UPDATE users
    SET display_name = @display_name,
        updated_at   = now()
    WHERE id = @id
    RETURNING id, email, display_name, role, created_at
)
SELECT u.id, u.email, u.display_name, u.role, u.created_at, a.updated_at AS avatar_updated_at
FROM updated u
LEFT JOIN user_avatars a ON a.user_id = u.id;
