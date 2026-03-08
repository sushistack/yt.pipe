# Story 3.1: Image Prompt Generation & Sanitization

Status: done

## Story
As a creator, I want the system to auto-generate image prompts from the scenario's visual descriptions with safety processing so that each scene gets a high-quality, API-safe image prompt.

## Implementation
- `internal/service/image_prompt.go`: GenerateImagePrompts() applies Go text/template to visual descriptions, sanitizePrompt() removes dangerous terms via regex and appends safety modifiers
- `internal/service/image_prompt_test.go`: 13 tests covering generation, sanitization, case handling, word boundaries, custom modifiers/terms, external templates, template versioning
- ImagePromptConfig: template path, dangerous terms, safety modifiers (all configurable)
- Default dangerous terms: gore, blood, violent, gruesome, mutilation, decapitation, dismemberment
- Default safety modifiers: digital illustration, safe for work, artistic style, clean composition
- Template version tracked via SHA-256 hash

## Dev Agent Record
### Agent Model Used
Claude Opus 4.6
