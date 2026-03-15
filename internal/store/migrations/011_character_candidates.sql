CREATE TABLE IF NOT EXISTS character_candidates (
    id            TEXT PRIMARY KEY,
    project_id    TEXT NOT NULL,
    scp_id        TEXT NOT NULL,
    candidate_num INTEGER NOT NULL,
    image_path    TEXT NOT NULL DEFAULT '',
    description   TEXT NOT NULL DEFAULT '',
    status        TEXT NOT NULL DEFAULT 'pending',
    error_detail  TEXT NOT NULL DEFAULT '',
    created_at    TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_character_candidates_project ON character_candidates(project_id);
