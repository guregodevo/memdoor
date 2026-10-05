-- Add role column to users table
-- Roles: 'admin' (workspace administrator) or 'user' (regular user)
-- Default to 'user' for all existing users
-- First user in workspace or specific bootstrap emails will be promoted to admin

ALTER TABLE users ADD COLUMN role TEXT NOT NULL DEFAULT 'user';

-- Create index for efficient role-based queries
CREATE INDEX idx_users_role ON users(role);

-- Update first user in default workspace to admin (bootstrap)
UPDATE users
SET role = 'admin'
WHERE id = (
    SELECT id FROM users
    WHERE workspace_id = '00000000-0000-0000-0000-000000000001'
    ORDER BY created_at ASC
    LIMIT 1
);

-- Also make admin@localhost an admin if it exists (setup script creates this)
UPDATE users
SET role = 'admin'
WHERE email = 'admin@localhost';
