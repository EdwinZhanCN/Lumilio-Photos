ALTER TABLE users
    ADD COLUMN display_name VARCHAR(100) NOT NULL DEFAULT '',
    ADD COLUMN avatar_url TEXT,
    ADD COLUMN role VARCHAR(20) NOT NULL DEFAULT 'user'
        CHECK (role IN ('admin', 'user'));

UPDATE users
SET display_name = username
WHERE display_name = '';

WITH first_user AS (
    SELECT user_id
    FROM users
    ORDER BY created_at ASC, user_id ASC
    LIMIT 1
)
UPDATE users
SET role = 'admin'
WHERE user_id IN (SELECT user_id FROM first_user);

CREATE INDEX idx_users_role ON users(role);
