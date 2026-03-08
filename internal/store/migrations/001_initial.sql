CREATE TABLE IF NOT EXISTS projects (
    id TEXT PRIMARY KEY,
    scp_id TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    scene_count INTEGER NOT NULL DEFAULT 0,
    workspace_path TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_projects_scp_id ON projects(scp_id);
CREATE INDEX idx_projects_status ON projects(status);

CREATE TABLE IF NOT EXISTS jobs (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id),
    type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    progress INTEGER NOT NULL DEFAULT 0,
    result TEXT,
    error TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_jobs_project_id ON jobs(project_id);
CREATE INDEX idx_jobs_status ON jobs(status);

CREATE TABLE IF NOT EXISTS scene_manifests (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id TEXT NOT NULL REFERENCES projects(id),
    scene_num INTEGER NOT NULL,
    content_hash TEXT NOT NULL DEFAULT '',
    image_hash TEXT NOT NULL DEFAULT '',
    audio_hash TEXT NOT NULL DEFAULT '',
    subtitle_hash TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending',
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(project_id, scene_num)
);
CREATE INDEX idx_scene_manifests_project_id ON scene_manifests(project_id);

CREATE TABLE IF NOT EXISTS execution_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id TEXT NOT NULL REFERENCES projects(id),
    job_id TEXT REFERENCES jobs(id),
    stage TEXT NOT NULL,
    action TEXT NOT NULL,
    status TEXT NOT NULL,
    duration_ms INTEGER,
    estimated_cost_usd REAL,
    details TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_execution_logs_project_id ON execution_logs(project_id);
CREATE INDEX idx_execution_logs_job_id ON execution_logs(job_id);
