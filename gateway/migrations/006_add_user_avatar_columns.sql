-- Add avatar_emoji and icon columns to users table for agent avatars
-- These columns allow agents (stored in users table) to have custom avatars
ALTER TABLE users ADD COLUMN avatar_emoji TEXT;
ALTER TABLE users ADD COLUMN icon TEXT;
