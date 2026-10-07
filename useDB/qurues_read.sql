-- ============================================================================
-- READ QUERIES (Replica Node)
-- ============================================================================

-- name: GetUserByID :one
SELECT id, email, full_name
FROM users
WHERE id = $1;

-- name: GetUserByEmail :one
SELECT id, email, password_hash, full_name
FROM users
WHERE email = $1;

-- name: GetProductByID :one
SELECT id, name, price
FROM products
WHERE id = $1;

-- name: ListProducts :many
SELECT p.id, p.name, p.price, COALESCE(s.quantity, 0) AS stock_quantity
FROM products p
LEFT JOIN stock s ON p.id = s.product_id
ORDER BY p.name ASC;

-- name: GetOrderDetails :one
SELECT o.id, o.user_id, o.status, o.total_amount, o.created_at
FROM orders o
WHERE o.id = $1;

-- name: GetOrderItems :many
SELECT oi.id, oi.product_id, p.name AS product_name, oi.unit_price, oi.quantity, (oi.unit_price * oi.quantity) AS subtotal
FROM order_items oi
JOIN products p ON oi.product_id = p.id
WHERE oi.order_id = $1;

-- name: GetUserOrders :many
SELECT id, status, total_amount, created_at
FROM orders
WHERE user_id = $1
ORDER BY created_at DESC;
