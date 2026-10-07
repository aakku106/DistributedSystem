# Database Schema & SQL Query Documentation

This document provides an overview of the PostgreSQL database tables and the corresponding `sqlc` queries used for the starter e-commerce project.

---

## 1. Schema Overview & Table Descriptions

### `users`
* **Purpose**: Stores account details for users in the system.
* **Key Behavior**: Serves as the primary entity for ownership across orders.
* **Columns**:
  * `id` (`UUID`): Primary key, auto-generated using `gen_random_uuid()`.
  * `email` (`VARCHAR(255)`): Unique email address for authentication/identification.
  * `password_hash` (`VARCHAR(255)`): Hashed user password.
  * `full_name` (`VARCHAR(100)`): Display name of the user.

---

### `products`
* **Purpose**: Holds basic catalog details for items available for sale.
* **Key Behavior**: Stock tracking is decoupled into its own table (`stock`) to prevent lock contention during inventory updates.
* **Columns**:
  * `id` (`UUID`): Primary key, auto-generated.
  * `name` (`VARCHAR(255)`): Name of the product.
  * `price` (`NUMERIC(12, 2)`): Base price of the product (must be $\ge 0$).

---

### `stock`
* **Purpose**: Manages real-time inventory counts for products in a 1:1 relationship.
* **Key Behavior**: Referenced via `product_id`. Automatically cleaned up if the corresponding product is deleted (`ON DELETE CASCADE`).
* **Columns**:
  * `product_id` (`UUID`): Primary key and foreign key referencing `products(id)`.
  * `quantity` (`INT`): Available units in stock (must be $\ge 0$).
  * `updated_at` (`TIMESTAMPTZ`): Timestamp tracking the last inventory alteration.

---

### `orders`
* **Purpose**: Acts as the header table for purchase orders placed by users.
* **Key Behavior**: Tracks overall order lifecycle (`PENDING`, `PAID`, `CANCELLED`, etc.) and total cost. Deleted cascade-wise if the associated user is removed.
* **Columns**:
  * `id` (`UUID`): Primary key, auto-generated.
  * `user_id` (`UUID`): Foreign key referencing `users(id)`.
  * `status` (`VARCHAR(30)`): Order lifecycle state (default: `'PENDING'`).
  * `total_amount` (`NUMERIC(12, 2)`): Total calculated price of the order.
  * `created_at` (`TIMESTAMPTZ`): Creation timestamp.

---

### `order_items`
* **Purpose**: Represents line items attached to a specific order.
* **Key Behavior**: Snapshots `unit_price` at the time of order creation to maintain historical accuracy even if the product's catalog price changes later.
* **Columns**:
  * `id` (`UUID`): Primary key, auto-generated.
  * `order_id` (`UUID`): Foreign key referencing `orders(id)` (`ON DELETE CASCADE`).
  * `product_id` (`UUID`): Foreign key referencing `products(id)`.
  * `unit_price` (`NUMERIC(12, 2)`): Price of a single unit at purchase time.
  * `quantity` (`INT`): Number of units purchased (must be $> 0$).

---

## 2. SQL Query Operations (`sqlc`)

Below is the detailed breakdown of what each annotated query accomplishes and how it interacts with the database.

---

### User Queries

#### `CreateUser` (`:one`)
* **Table Target**: `users`
* **Operation**: `INSERT`
* **Description**: Registers a new user account. Returns the created `id`, `email`, and `full_name` to pass back to the caller.

#### `UpdateUser` (`:one`)
* **Table Target**: `users`
* **Operation**: `UPDATE`
* **Description**: Updates user profile details (`email`, `full_name`). Uses `COALESCE` to allow partial updates without overwriting existing data if `NULL` parameters are passed.

#### `DeleteUser` (`:exec`)
* **Table Target**: `users`
* **Operation**: `DELETE`
* **Description**: Deletes a user by their `id`. Triggers cascading deletion on any associated orders.

---

### Product Queries

#### `CreateProduct` (`:one`)
* **Table Target**: `products`
* **Operation**: `INSERT`
* **Description**: Inserts a new product into the catalog with its `name` and `price`. Returns the newly generated product record.

#### `UpdateProduct` (`:one`)
* **Table Target**: `products`
* **Operation**: `UPDATE`
* **Description**: Modifies a product's name or price based on its `id`.

#### `DeleteProduct` (`:exec`)
* **Table Target**: `products`
* **Operation**: `DELETE`
* **Description**: Removes a product from the database. Automatically removes the associated `stock` record via cascading foreign keys.

---

### Stock Queries

#### `CreateStock` (`:one`)
* **Table Target**: `stock`
* **Operation**: `INSERT`
* **Description**: Initializes an inventory row for a newly created product (`product_id`) with an initial `quantity`.

#### `UpdateStock` (`:one`)
* **Table Target**: `stock`
* **Operation**: `UPDATE`
* **Description**: Overwrites the stock quantity to a specific target number (e.g., during manual inventory reconciliations) and updates `updated_at`.

#### `DeductStock` (`:one`)
* **Table Target**: `stock`
* **Operation**: `UPDATE`
* **Description**: Atomic deduction of stock for checkout processing. The `quantity >= $2` clause guarantees that stock cannot drop below zero, failing the update if insufficient inventory exists.

---

### Order Queries

#### `CreateOrder` (`:one`)
* **Table Target**: `orders`
* **Operation**: `INSERT`
* **Description**: Creates a new order header for a user with an initial status and total price.

#### `CreateOrderItem` (`:one`)
* **Table Target**: `order_items`
* **Operation**: `INSERT`
* **Description**: Adds an individual line item to an order, capturing the historical snapshot of the unit price and quantity.

#### `UpdateOrderStatus` (`:one`)
* **Table Target**: `orders`
* **Operation**: `UPDATE`
* **Description**: Changes the order status (e.g., from `'PENDING'` to `'PAID'` or `'SHIPPED'`).

#### `CancelOrder` (`:one`)
* **Table Target**: `orders`
* **Operation**: `UPDATE`
* **Description**: Sets the order status to `'CANCELLED'`. Guarded by `status != 'CANCELLED'` to prevent duplicate cancellations.

#### `RestoreStockFromOrder` (`:exec`)
* **Table Target**: `stock`, `order_items`
* **Operation**: `UPDATE ... FROM`
* **Description**: Reverts stock levels when an order is cancelled. Joins `order_items` with `stock` for the target `order_id` and adds the ordered quantities back into inventory.