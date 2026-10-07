-- name: CreateUser :one
INSERT INTO users (
    email,
    password_hash,
    full_name
) VALUES (
    $1, $2, $3
)
RETURNING id, email, full_name;

-- name: UpdateUser :one
UPDATE users
SET
    email = COALESCE($2, email),
    full_name = COALESCE($3, full_name)
WHERE id = $1
RETURNING id, email, full_name;
