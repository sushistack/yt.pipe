# Story 20.3: Image Validator Service Core (EFR3)

Status: done

## Story

As a system,
I want an `ImageValidatorService` that evaluates generated images via multimodal LLM,
So that image quality can be automatically assessed against prompts and character references.

## Acceptance Criteria

1. `ImageValidatorService` struct with `llm.LLM` and `*slog.Logger` dependencies
2. `ValidateImage(ctx, imagePath, originalPrompt, characterRefs []imagegen.CharacterRef) (*domain.ValidationResult, error)` reads image, base64-encodes, calls `CompleteWithVision()`, parses JSON response
3. Evaluation prompt requests JSON with `prompt_match`, `character_match`, `technical_score`, `reasons` fields
4. When LLM returns `llm.ErrNotSupported`, validation returns `nil, nil` (skip) with warning log
5. When LLM returns malformed JSON, returns error with raw response for debugging
6. When scene has no character references, `CharacterMatch` is set to `-1` and excluded from weighted average
7. When image file does not exist, returns error without calling LLM

## Tasks / Subtasks

- [x] Task 1: Create `ImageValidatorService` struct and constructor (AC: #1)
  - [x] 1.1 Create `internal/service/image_validator.go` with `ImageValidatorService` struct
  - [x] 1.2 Add `NewImageValidatorService(llm llm.LLM, logger *slog.Logger)` constructor

- [x] Task 2: Implement `ValidateImage()` core logic (AC: #2, #3, #6, #7)
  - [x] 2.1 Read image file and base64-encode with MIME type detection
  - [x] 2.2 Build structured evaluation prompt with system + user VisionMessages
  - [x] 2.3 Call `CompleteWithVision()` and parse JSON response into `ValidationResult`
  - [x] 2.4 Handle no-character case: set `CharacterMatch = -1` when `characterRefs` is empty

- [x] Task 3: Handle error cases (AC: #4, #5, #7)
  - [x] 3.1 Return `nil, nil` on `llm.ErrNotSupported` with warning log
  - [x] 3.2 Return error with raw LLM response on malformed JSON
  - [x] 3.3 Return error on missing image file without calling LLM

- [x] Task 4: Add unit tests
  - [x] 4.1 Test successful validation with character refs (mock LLM returns valid JSON)
  - [x] 4.2 Test successful validation without character refs (CharacterMatch == -1)
  - [x] 4.3 Test `ErrNotSupported` returns nil, nil
  - [x] 4.4 Test malformed JSON returns error with raw response
  - [x] 4.5 Test missing image file returns error
  - [x] 4.6 Test evaluation prompt format (verify VisionMessage structure)

## Dev Notes

### Service Pattern

Follow `ImageGenService` pattern in `internal/service/image_gen.go`:
- Struct with plugin interface + logger dependencies
- Constructor: `NewImageValidatorService(llm llm.LLM, logger *slog.Logger)`
- No store dependency — score persistence is handled by caller (Story 20-4/20-5)

```go
type ImageValidatorService struct {
    llm    llm.LLM
    logger *slog.Logger
}
```

### Image Base64 Encoding

```go
data, err := os.ReadFile(imagePath)
// Detect MIME from extension
ext := filepath.Ext(imagePath)
mime := "image/png" // default
switch ext {
case ".jpg", ".jpeg": mime = "image/jpeg"
case ".webp": mime = "image/webp"
}
dataURI := fmt.Sprintf("data:%s;base64,%s", mime, base64.StdEncoding.EncodeToString(data))
```

### Evaluation Prompt Structure

System message (text only):
```
You are an image quality evaluator for SCP content.
Evaluate the image against the following criteria:
1. Prompt consistency (0-100): Does the image match the visual description?
2. Character appearance (0-100): Does the character match the reference? (-1 if no character)
3. Technical quality (0-100): Are there distortions, artifacts, or rendering errors?

Return ONLY valid JSON: {"prompt_match": N, "character_match": N, "technical_score": N, "reasons": ["..."]}
```

User message (multimodal — text + image):
```
Original prompt: {prompt}
Character references: {character descriptions or "None"}
```
+ Image content part with base64 data URI

### JSON Response Parsing

```go
type validationResponse struct {
    PromptMatch    int      `json:"prompt_match"`
    CharacterMatch int      `json:"character_match"`
    TechnicalScore int      `json:"technical_score"`
    Reasons        []string `json:"reasons"`
}
```

Use `extractJSON()` from `openai.go` pattern to strip markdown code fences before parsing. Reuse or duplicate the 10-line helper.

### Error Handling Matrix

| Condition | Action |
|-----------|--------|
| Image file not found | Return `fmt.Errorf("image file not found: %s", path)` — don't call LLM |
| `llm.ErrNotSupported` | Return `nil, nil` + `logger.Warn("vision not supported, skipping validation")` |
| Malformed JSON | Return `fmt.Errorf("parse validation response: %w (raw: %s)", err, rawContent)` |
| LLM API error | Return wrapped error from LLM call |
| No character refs | Set `CharacterMatch = -1` in result, call `Evaluate()` |

### Previous Story Outputs (20-1, 20-2)

- `llm.LLM` interface has `CompleteWithVision(ctx, []VisionMessage, CompletionOptions)`
- `llm.ErrNotSupported` sentinel for unsupported providers
- `llm.VisionMessage{Role, Content []ContentPart}` and `llm.ContentPart{Type, Text, ImageURL}`
- `domain.ValidationResult` with `CalculateScore()` and `Evaluate(threshold)`
- `imagegen.CharacterRef{Name, VisualDescriptor, ImagePromptBase, StyleGuide}`

### Project Structure Notes

Files to create:
1. `internal/service/image_validator.go` — service implementation
2. `internal/service/image_validator_test.go` — unit tests

### References

- [Source: _bmad-output/planning-artifacts/architecture.md#EFR3 lines 1381-1466]
- [Source: _bmad-output/planning-artifacts/epics.md#Story 20.3 lines 3641-3672]
- [Source: internal/service/image_gen.go — service pattern]
- [Source: internal/plugin/llm/interface.go — VisionMessage, CompleteWithVision]
- [Source: internal/domain/validation.go — ValidationResult]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

### Completion Notes List

- ImageValidatorService with ValidateImage() — reads image, base64-encodes, sends via CompleteWithVision
- Structured evaluation prompt requesting JSON with prompt_match, character_match, technical_score, reasons
- Error handling: ErrNotSupported → nil,nil; malformed JSON → error with raw; missing file → error without LLM call
- No-character scenes: CharacterMatch forced to -1 regardless of LLM response
- extractValidationJSON helper strips markdown code fences
- 8 tests: success with/without chars, ErrNotSupported, malformed JSON, missing file, message structure, code fence, MIME detect

### Change Log

- 2026-03-18: Story 20.3 implementation complete

### File List

- internal/service/image_validator.go (new — ImageValidatorService + ValidateImage)
- internal/service/image_validator_test.go (new — 8 unit tests)
