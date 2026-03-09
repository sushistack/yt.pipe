CREATE TABLE characters (
    id                TEXT PRIMARY KEY,
    scp_id            TEXT NOT NULL,
    canonical_name    TEXT NOT NULL,
    aliases           TEXT NOT NULL DEFAULT '[]',
    visual_descriptor TEXT NOT NULL DEFAULT '',
    style_guide       TEXT NOT NULL DEFAULT '',
    image_prompt_base TEXT NOT NULL DEFAULT '',
    created_at        TEXT NOT NULL,
    updated_at        TEXT NOT NULL
);

CREATE INDEX idx_characters_scp_id ON characters(scp_id);
