-- Add review_token column to projects table for review dashboard authentication
ALTER TABLE projects ADD COLUMN review_token TEXT DEFAULT NULL;

-- Index for fast token lookups
CREATE INDEX IF NOT EXISTS idx_projects_review_token ON projects(review_token);
