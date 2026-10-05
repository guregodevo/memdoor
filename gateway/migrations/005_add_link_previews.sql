-- Add link_previews column to messages table
-- Link previews are stored as JSON array of LinkPreview objects
ALTER TABLE messages ADD COLUMN content_link_previews TEXT;
