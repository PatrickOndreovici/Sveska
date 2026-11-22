-- name: CreateSession :exec
INSERT INTO sessions (user_id, token_hash, expires_at)
VALUES (@user_id, @token_hash, @expires_at);

-- name: GetSessionUser :one
-- Returns the user behind an unexpired session.
SELECT u.id, u.email, u.display_name, u.role, u.created_at, a.updated_at AS avatar_updated_at
FROM sessions s
JOIN users u ON u.id = s.user_id
LEFT JOIN user_avatars a ON a.user_id = u.id
WHERE s.token_hash = @token_hash
  AND s.expires_at > now();

-- name: DeleteSession :exec
DELETE FROM sessions
WHERE token_hash = @token_hash;
