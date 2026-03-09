-- Scene approval tracking for per-scene image/TTS approval workflow (Epic 16)
CREATE TABLE IF NOT EXISTS scene_approvals (
    project_id TEXT NOT NULL,
    scene_num  INTEGER NOT NULL,
    asset_type TEXT NOT NULL CHECK (asset_type IN ('image', 'tts')),
    status     TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'generated', 'approved', 'rejected')),
    attempts   INTEGER NOT NULL DEFAULT 0,
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (project_id, scene_num, asset_type),
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_scene_approvals_project ON scene_approvals(project_id);
CREATE INDEX IF NOT EXISTS idx_scene_approvals_project_type ON scene_approvals(project_id, asset_type);
