CREATE TABLE IF NOT EXISTS bgms (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    file_path TEXT NOT NULL,
    mood_tags TEXT NOT NULL DEFAULT '[]',
    duration_ms INTEGER NOT NULL DEFAULT 0,
    license_type TEXT NOT NULL CHECK(license_type IN ('royalty_free', 'cc_by', 'cc_by_sa', 'cc_by_nc', 'custom')),
    license_source TEXT NOT NULL DEFAULT '',
    credit_text TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_bgms_mood_tags ON bgms(mood_tags);

CREATE TABLE IF NOT EXISTS scene_bgm_assignments (
    project_id TEXT NOT NULL,
    scene_num INTEGER NOT NULL,
    bgm_id TEXT NOT NULL REFERENCES bgms(id),
    volume_db REAL NOT NULL DEFAULT 0,
    fade_in_ms INTEGER NOT NULL DEFAULT 2000,
    fade_out_ms INTEGER NOT NULL DEFAULT 2000,
    ducking_db REAL NOT NULL DEFAULT -12,
    auto_recommended INTEGER NOT NULL DEFAULT 0,
    confirmed INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (project_id, scene_num)
);

CREATE INDEX IF NOT EXISTS idx_scene_bgm_assignments_bgm ON scene_bgm_assignments(bgm_id);
