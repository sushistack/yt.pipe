CREATE TABLE IF NOT EXISTS shot_manifests (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id TEXT NOT NULL REFERENCES projects(id),
    scene_num INTEGER NOT NULL,
    shot_num INTEGER NOT NULL,
    content_hash TEXT NOT NULL DEFAULT '',
    image_hash TEXT NOT NULL DEFAULT '',
    gen_method TEXT NOT NULL DEFAULT 'text_to_image',
    status TEXT NOT NULL DEFAULT 'pending',
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(project_id, scene_num, shot_num)
);
