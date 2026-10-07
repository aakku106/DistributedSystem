-- ============================================================================
-- WRITE QUERIES (Primary Node)
-- ============================================================================

-- name: CreateUser :one
INSERT INTO users (email, password_hash, full_name)
VALUES ($1, $2, $3)
RETURNING id, email, full_name;

-- name: UpdateUser :one
UPDATE users
SET email = COALESCE($2, email),
    full_name = COALESCE($3, full_name)
WHERE id = $1
RETURNING id, email, full_name;

-- name: CreateProduct :one
INSERT INTO products (name, price)
VALUES ($1, $2)
RETURNING id, name, price;

-- name: UpdateStock :one
UPDATE stock
SET quantity = $2,
    updated_at = CURRENT_TIMESTAMP
WHERE product_id = $1
RETURNING product_id, quantity, updated_at;

-- name: DeductStock :one
UPDATE stock
SET quantity = quantity - $2,
    updated_at = CURRENT_TIMESTAMP
WHERE product_id = $1 AND quantity >= $2
RETURNING product_id, quantity, updated_at;

-- name: CreateOrder :one
INSERT INTO orders (user_id, status, total_amount)
VALUES ($1, $2, $3)
RETURNING id, user_id, status, total_amount, created_at;

-- name: CreateOrderItem :one
INSERT INTO order_items (order_id, product_id, unit_price, quantity)
VALUES ($1, $2, $3, $4)
RETURNING id, order_id, product_id, unit_price, quantity;

-- name: UpdateOrderStatus :one
UPDATE orders
SET status = $2
WHERE id = $1
RETURNING id, status;
