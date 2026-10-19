-- name: CreateUser :one
INSERT INTO users (email, password_hash, display_name, role)
VALUES (@email, @password_hash, @display_name, @role)
RETURNING id, email, display_name, role, created_at;

-- name: GetUserCredentialsByEmail :one
SELECT id, email, password_hash, display_name, role, created_at
FROM users
WHERE email = @email;
