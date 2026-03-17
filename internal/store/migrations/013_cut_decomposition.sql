ALTER TABLE shot_manifests ADD COLUMN sentence_start INTEGER NOT NULL DEFAULT 0;
ALTER TABLE shot_manifests ADD COLUMN sentence_end INTEGER NOT NULL DEFAULT 0;
ALTER TABLE shot_manifests ADD COLUMN cut_num INTEGER NOT NULL DEFAULT 0;

-- Migrate existing data: shot_num becomes sentence_start, sentence_end = sentence_start, cut_num = 1
UPDATE shot_manifests SET sentence_start = shot_num, sentence_end = shot_num, cut_num = 1 WHERE sentence_start = 0;

-- New UNIQUE index for 3-level key queries (keep old UNIQUE intact for SQLite compat)
CREATE UNIQUE INDEX IF NOT EXISTS idx_shot_manifests_cut
    ON shot_manifests(project_id, scene_num, sentence_start, cut_num);
