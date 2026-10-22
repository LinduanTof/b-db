PRAGMA foreign_keys = ON;

CREATE TABLE users (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT    NOT NULL,
    email       TEXT    NOT NULL UNIQUE,
    active      INTEGER NOT NULL DEFAULT 1,
    balance     REAL    NOT NULL DEFAULT 0,
    note        TEXT,
    created_at  TEXT    NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE orders (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id     INTEGER NOT NULL REFERENCES users(id),
    amount      REAL    NOT NULL,
    status      TEXT    NOT NULL DEFAULT 'pending',
    created_at  TEXT    NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_orders_user_id ON orders(user_id);
CREATE INDEX idx_orders_status ON orders(status);

CREATE VIEW active_users AS
SELECT id, name, email
FROM users
WHERE active = 1;

INSERT INTO users (name, email, active, balance, note) VALUES
    ('张三', 'zhangsan@example.com', 1, 128.50, '长期客户'),
    ('李四', 'lisi@example.com', 1, 0, NULL),
    ('Alice Smith', 'alice@example.com', 1, 999.99, 'vip'),
    ('Bob Jones', 'bob@example.com', 0, 12.30, 'inactive since 2025'),
    ('王五', 'wangwu@example.com', 1, 42.00, '中文备注：含逗号,和"引号"');

INSERT INTO orders (user_id, amount, status) VALUES
    (1, 59.90, 'paid'),
    (1, 68.60, 'pending'),
    (3, 500.00, 'paid'),
    (4, 12.30, 'refunded'),
    (5, 42.00, 'paid');
