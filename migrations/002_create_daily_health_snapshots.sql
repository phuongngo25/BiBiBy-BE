-- 002_create_daily_health_snapshots.sql
-- Create daily_health_snapshots table to store frozen daily user targets

CREATE TABLE IF NOT EXISTS daily_health_snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    snapshot_date DATE NOT NULL,
    weight_kg DOUBLE PRECISION NOT NULL,
    activity_level VARCHAR(50) NOT NULL,
    goal_type VARCHAR(50) NOT NULL,
    bmr INTEGER NOT NULL,
    tdee INTEGER NOT NULL,
    target_calories INTEGER NOT NULL,
    target_water INTEGER NOT NULL,
    goal_strategy_version VARCHAR(20) NOT NULL DEFAULT 'v1',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Ensure user cannot have duplicate snapshots for the same calendar date
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_date ON daily_health_snapshots (user_id, snapshot_date);
