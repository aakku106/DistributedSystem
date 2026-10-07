# Database Schema & SQL Query Reference

This document provides a complete reference for the PostgreSQL e-commerce starter schema and its corresponding `sqlc` queries, tailored for a **Primary-Replica (Read-Write Split)** architecture.

---

## 1. Business & Entity Overview

From a business operations perspective, the tables represent the following core entities:

* **`users` (Customer Registry)**: Holds customer account details, authentication data, and personal information.
* **`products` (Product Catalog)**: Represents items displayed in the storefront (e.g., Toy Aeroplane, Fighter Jet) along with their current listing prices.
* **`stock` (Physical Inventory)**: Tracks the physical count of available items in the warehouse in a 1:1 relationship with `products`. Decoupled from `products` to minimize row locking during frequent stock updates.
* **`orders` (Sales Order Header)**: Stores receipt-level details for sales made to customers (customer ID, order status, total price, timestamp).
* **`order_items` (Sales Line Items)**: Tracks individual products and quantities purchased inside a specific order.
  * **Note**: This reflects **customer purchases**, not supplier/procurement details. It snapshots `unit_price` at the moment of sale so historical records remain unaffected by future product catalog price changes.

---

## 2. Table Definitions

### `users`
```sql
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    full_name VARCHAR(100) NOT NULL
);
```

### `products`
```sql
CREATE TABLE products (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    price NUMERIC(12, 2) NOT NULL CHECK (price >= 0)
);
```

### `stock`
```sql
CREATE TABLE stock (
    product_id UUID PRIMARY KEY REFERENCES products(id) ON DELETE CASCADE,
    quantity INT NOT NULL DEFAULT 0 CHECK (quantity >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

### `orders`
```sql
CREATE TABLE orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status VARCHAR(30) NOT NULL DEFAULT 'PENDING',
    total_amount NUMERIC(12, 2) NOT NULL CHECK (total_amount >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

### `order_items`
```sql
CREATE TABLE order_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    product_id UUID NOT NULL REFERENCES products(id),
    unit_price NUMERIC(12, 2) NOT NULL CHECK (unit_price >= 0),
    quantity INT NOT NULL CHECK (quantity > 0)
);
```

---

## 3. Read-Write Split Query Architecture

To support **Read Replicas**, queries are explicitly split into two categories:
* **Write Queries (`queries_write.sql`)**: Routed to the **Primary Database**.
* **Read Queries (`queries_read.sql`)**: Routed to the **Read-Only Replica Database**.

---

### A. Primary / Write Queries (`queries_write.sql`)

All data-modifying queries (`INSERT`, `UPDATE`, `DELETE`) and immediate state-changing operations run on the Primary DB.

#### User Writes
```sql
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

-- name: DeleteUser :exec
DELETE FROM users
WHERE id = $1;
```

#### Product Writes
```sql
-- name: CreateProduct :one
INSERT INTO products (name, price)
VALUES ($1, $2)
RETURNING id, name, price;

-- name: UpdateProduct :one
UPDATE products
SET name = $2, price = $3
WHERE id = $1
RETURNING id, name, price;

-- name: DeleteProduct :exec
DELETE FROM products
WHERE id = $1;
```

#### Stock Writes
```sql
-- name: CreateStock :one
INSERT INTO stock (product_id, quantity)
VALUES ($1, $2)
RETURNING product_id, quantity, updated_at;

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
WHERE product_id = $1 
  AND quantity >= $2
RETURNING product_id, quantity, updated_at;
```

#### Order Writes
```sql
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

-- name: CancelOrder :one
UPDATE orders
SET status = 'CANCELLED'
WHERE id = $1 
  AND status != 'CANCELLED'
RETURNING id, status;

-- name: RestoreStockFromOrder :exec
UPDATE stock s
SET quantity = s.quantity + oi.quantity,
    updated_at = CURRENT_TIMESTAMP
FROM order_items oi
WHERE oi.product_id = s.product_id
  AND oi.order_id = $1;
```

---

### B. Secondary / Read Queries (`queries_read.sql`)

All retrieval queries (`SELECT`) offload read traffic onto the secondary read replica.

#### User Reads
```sql
-- name: GetUserByID :one
SELECT id, email, full_name
FROM users
WHERE id = $1;

-- name: GetUserByEmail :one
SELECT id, email, password_hash, full_name
FROM users
WHERE email = $1;
```

#### Product & Stock Reads
```sql
-- name: GetProductByID :one
SELECT id, name, price
FROM products
WHERE id = $1;

-- name: ListProductsWithStock :many
SELECT 
    p.id, 
    p.name, 
    p.price, 
    COALESCE(s.quantity, 0) AS stock_quantity
FROM products p
LEFT JOIN stock s ON p.id = s.product_id
ORDER BY p.name ASC;
```

#### Order Reads
```sql
-- name: GetOrderHeader :one
SELECT id, user_id, status, total_amount, created_at
FROM orders
WHERE id = $1;

-- name: GetOrderItemsDetails :many
SELECT 
    oi.id, 
    oi.product_id, 
    p.name AS product_name, 
    oi.unit_price, 
    oi.quantity, 
    (oi.unit_price * oi.quantity) AS line_total
FROM order_items oi
JOIN products p ON oi.product_id = p.id
WHERE oi.order_id = $1;

-- name: ListUserOrders :many
SELECT id, status, total_amount, created_at
FROM orders
WHERE user_id = $1
ORDER BY created_at DESC;
```

---

## 4. Key Architectural Considerations

1. **Replication Lag Handling**:
   - Because replication from Primary to Replica is asynchronous, querying a Replica immediately after a write (e.g., redirecting to order details immediately upon checkout) may cause a transient `404 Not Found`.
   - **Rule**: Read requests that immediately follow a write operation should route through the **Primary DB pool** to guarantee immediate consistency.

2. **Transactional Integrity**:
   - Multi-step operations (e.g., `CreateOrder` + `CreateOrderItem` + `DeductStock`) **must** be executed entirely within a single transaction targeting the **Primary DB pool**.