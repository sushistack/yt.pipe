CREATE TABLE mood_presets (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    speed       REAL NOT NULL DEFAULT 1.0,
    emotion     TEXT NOT NULL DEFAULT 'neutral',
    pitch       REAL NOT NULL DEFAULT 1.0,
    params_json TEXT NOT NULL DEFAULT '{}',
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

CREATE TABLE scene_mood_assignments (
    project_id  TEXT NOT NULL,
    scene_num   INTEGER NOT NULL,
    preset_id   TEXT NOT NULL REFERENCES mood_presets(id),
    auto_mapped INTEGER NOT NULL DEFAULT 0,
    confirmed   INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (project_id, scene_num)
);

CREATE INDEX idx_scene_mood_assignments_preset ON scene_mood_assignments(preset_id);
