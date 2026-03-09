CREATE TABLE prompt_templates (
    id          TEXT PRIMARY KEY,
    category    TEXT NOT NULL CHECK(category IN ('scenario','image','tts','caption')),
    name        TEXT NOT NULL,
    content     TEXT NOT NULL,
    version     INTEGER NOT NULL DEFAULT 1,
    is_default  INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

CREATE TABLE prompt_template_versions (
    id          TEXT PRIMARY KEY,
    template_id TEXT NOT NULL REFERENCES prompt_templates(id),
    version     INTEGER NOT NULL,
    content     TEXT NOT NULL,
    created_at  TEXT NOT NULL
);

CREATE TABLE project_template_overrides (
    project_id  TEXT NOT NULL,
    template_id TEXT NOT NULL,
    content     TEXT NOT NULL,
    created_at  TEXT NOT NULL,
    PRIMARY KEY (project_id, template_id)
);

CREATE INDEX idx_templates_category ON prompt_templates(category);
CREATE INDEX idx_template_versions_template_id ON prompt_template_versions(template_id);
