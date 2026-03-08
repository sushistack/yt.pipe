CREATE TABLE IF NOT EXISTS feedback (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id TEXT NOT NULL REFERENCES projects(id),
    scene_num INTEGER NOT NULL,
    asset_type TEXT NOT NULL,  -- 'image', 'audio', 'subtitle', 'scenario'
    rating TEXT NOT NULL,      -- 'good', 'bad', 'neutral'
    comment TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_feedback_project_id ON feedback(project_id);
CREATE INDEX idx_feedback_asset_type ON feedback(asset_type);
