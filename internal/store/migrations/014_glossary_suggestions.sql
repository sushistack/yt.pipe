-- Glossary suggestion tracking for LLM-based term extraction (Epic 19, EFR2)
CREATE TABLE IF NOT EXISTS glossary_suggestions (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id    TEXT NOT NULL,
    term          TEXT NOT NULL,
    pronunciation TEXT NOT NULL,
    definition    TEXT NOT NULL DEFAULT '',
    category      TEXT NOT NULL DEFAULT '',
    status        TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected')),
    created_at    TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at    TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(term, project_id),
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_glossary_suggestions_project ON glossary_suggestions(project_id);
CREATE INDEX IF NOT EXISTS idx_glossary_suggestions_status ON glossary_suggestions(status);
