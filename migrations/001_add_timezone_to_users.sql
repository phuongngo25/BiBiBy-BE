-- 001_add_timezone_to_users.sql
-- Add timezone column to users if not exists, default to 'UTC'
ALTER TABLE users ADD COLUMN IF NOT EXISTS timezone VARCHAR(50) DEFAULT 'UTC';

-- Backfill existing users timezone to 'Asia/Ho_Chi_Minh'
UPDATE users SET timezone = 'Asia/Ho_Chi_Minh' WHERE timezone IS NULL;
