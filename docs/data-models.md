# Data Models — yt.pipe

> Auto-generated project documentation (2026-03-09)

## Overview

SQLite database with 7 migrations, 13+ tables, pure-Go driver (`modernc.org/sqlite`), WAL mode, foreign keys enabled. Migrations are embedded via `//go:embed` and run automatically on startup.

## Database Schema

### Core Tables (Migration 001)

#### projects
| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | TEXT | PRIMARY KEY | UUID |
| scp_id | TEXT | NOT NULL | SCP entity ID (e.g., SCP-173) |
| status | TEXT | NOT NULL, DEFAULT 'pending' | Workflow state |
| scene_count | INTEGER | DEFAULT 0 | Number of scenes |
| workspace_path | TEXT | NOT NULL | Filesystem path |
| created_at | TEXT | NOT NULL, DEFAULT now | RFC3339 |
| updated_at | TEXT | NOT NULL, DEFAULT now | RFC3339 |

Indexes: `idx_projects_scp_id`, `idx_projects_status`

#### jobs
| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | TEXT | PRIMARY KEY | UUID |
| project_id | TEXT | FK→projects | Parent project |
| type | TEXT | NOT NULL | Job type |
| status | TEXT | DEFAULT 'pending' | Job status |
| progress | INTEGER | DEFAULT 0 | 0-100% |
| result | TEXT | nullable | JSON result |
| error | TEXT | nullable | Error message |

#### scene_manifests
| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | INTEGER | AUTOINCREMENT PK | |
| project_id | TEXT | FK→projects | |
| scene_num | INTEGER | NOT NULL | 1-indexed |
| content_hash | TEXT | DEFAULT '' | Scenario text hash |
| image_hash | TEXT | DEFAULT '' | Image data hash |
| audio_hash | TEXT | DEFAULT '' | TTS audio hash |
| subtitle_hash | TEXT | DEFAULT '' | Subtitle hash |
| status | TEXT | DEFAULT 'pending' | Build status |

UNIQUE(project_id, scene_num) — enables incremental build skip detection.

#### execution_logs
| Column | Type | Description |
|--------|------|-------------|
| id | INTEGER | AUTOINCREMENT PK |
| project_id | TEXT | FK→projects |
| job_id | TEXT | FK→jobs (optional) |
| stage | TEXT | Pipeline stage |
| action | TEXT | Action performed |
| duration_ms | INTEGER | Execution duration |
| estimated_cost_usd | REAL | API cost estimate |
| details | TEXT | JSON details |

### Feedback (Migration 002)

#### feedback
| Column | Type | Description |
|--------|------|-------------|
| id | INTEGER | AUTOINCREMENT PK |
| project_id | TEXT | FK→projects |
| scene_num | INTEGER | Scene number |
| asset_type | TEXT | image, audio, subtitle, scenario |
| rating | TEXT | good, bad, neutral |
| comment | TEXT | Optional |

### Prompt Templates (Migration 003)

#### prompt_templates
| Column | Type | Description |
|--------|------|-------------|
| id | TEXT | PK |
| category | TEXT | scenario, image, tts, caption |
| name | TEXT | NOT NULL |
| content | TEXT | Template text |
| version | INTEGER | DEFAULT 1 |
| is_default | INTEGER | 0/1 system default flag |

#### prompt_template_versions
Historical versioning with auto-pruning (10 most recent).

#### project_template_overrides
Per-project template customization. PK: (project_id, template_id).

### Characters (Migration 004)

#### characters
| Column | Type | Description |
|--------|------|-------------|
| id | TEXT | PK |
| scp_id | TEXT | SCP entity reference |
| canonical_name | TEXT | Primary name |
| aliases | TEXT | JSON array of alt names |
| visual_descriptor | TEXT | Physical description |
| style_guide | TEXT | Visual style guidelines |
| image_prompt_base | TEXT | Base prompt fragment |

### Mood Presets (Migration 005)

#### mood_presets
| Column | Type | Description |
|--------|------|-------------|
| id | TEXT | PK |
| name | TEXT | UNIQUE |
| speed | REAL | Speech rate (>0) |
| emotion | TEXT | neutral, angry, sad, etc. |
| pitch | REAL | Pitch multiplier (>0) |
| params_json | TEXT | Additional TTS params |

#### scene_mood_assignments
PK: (project_id, scene_num). Supports auto-mapping by LLM + user confirmation.

### Background Music (Migration 006)

#### bgms
| Column | Type | Description |
|--------|------|-------------|
| id | TEXT | PK |
| name | TEXT | NOT NULL |
| file_path | TEXT | Audio file path |
| mood_tags | TEXT | JSON array of mood keywords |
| duration_ms | INTEGER | Audio length |
| license_type | TEXT | royalty_free, cc_by, cc_by_sa, cc_by_nc, custom |
| credit_text | TEXT | Attribution line |

#### scene_bgm_assignments
PK: (project_id, scene_num). Per-scene volume, fade, ducking parameters.

### Scene Approvals (Migration 007)

#### scene_approvals
| Column | Type | Description |
|--------|------|-------------|
| project_id | TEXT | FK→projects (CASCADE) |
| scene_num | INTEGER | Scene number |
| asset_type | TEXT | image, tts |
| status | TEXT | pending, generated, approved, rejected |
| attempts | INTEGER | Regeneration count |

PK: (project_id, scene_num, asset_type)

---

## Project State Machine

```
pending → scenario_review → approved → image_review → tts_review → assembling → complete
                                    ↘ generating_assets → assembling (legacy skip-approval path)
```

### Valid Transitions
| From | To | Trigger |
|------|----|---------|
| pending | scenario_review | Scenario generated |
| scenario_review | approved | User approval |
| scenario_review | pending | Revert |
| approved | image_review | Image gen complete (approval path) |
| approved | generating_assets | Skip-approval path |
| image_review | tts_review | All images approved |
| tts_review | assembling | All TTS approved |
| generating_assets | assembling | Legacy path |
| assembling | complete | Assembly done |

## Scene Approval State Machine

```
pending → generated → approved (terminal)
                   → rejected → generated (retry)
```

## Domain Error Types

| Error | HTTP | Fields |
|-------|------|--------|
| NotFoundError | 404 | Resource, ID |
| ValidationError | 400 | Field, Message |
| PluginError | 500 | Plugin, Operation, Err |
| TransitionError | 409 | Current, Requested, Allowed[] |

## Store Operations Summary

- **80+ operations** across 10 store files
- Transactional: project deletion (cascade), template updates (version + prune)
- Pagination: `ListProjectsFiltered(state, scpID, limit, offset)` with count
- Search: `SearchCharactersByName` (case-insensitive, alias-aware), `SearchByMoodTags` (ranked)
