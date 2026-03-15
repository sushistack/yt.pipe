CREATE TABLE IF NOT EXISTS voice_cache (
    project_id  TEXT PRIMARY KEY,
    voice_id    TEXT NOT NULL,
    sample_path TEXT NOT NULL,
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

ALTER TABLE characters ADD COLUMN selected_image_path TEXT NOT NULL DEFAULT '';
