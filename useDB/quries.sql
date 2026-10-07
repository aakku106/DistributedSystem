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

-- name: DeleteUser :exec
DELETE FROM users
WHERE id = $1;


-- name: CreateProduct :one
INSERT INTO products (
    name, 
    price
) VALUES (
    $1, $2
)
RETURNING id, name, price;

-- name: UpdateProduct :one
UPDATE products
SET 
    name = $2,
    price = $3
WHERE id = $1
RETURNING id, name, price;

-- name: DeleteProduct :exec
DELETE FROM products
WHERE id = $1;
