---
title: 'Character Reference Image-Edit Implementation'
slug: 'character-ref-image-edit'
created: '2026-03-15'
status: 'completed'
stepsCompleted: [1, 2, 3, 4, 5]
tech_stack: ['Go 1.25.7', 'SiliconFlow REST API', 'Qwen/Qwen-Image-Edit', 'httptest', 'testify']
files_to_modify:
  - 'internal/plugin/imagegen/siliconflow.go'
  - 'internal/plugin/imagegen/siliconflow_test.go'
  - 'internal/plugin/imagegen/siliconflow_edit_poc_test.go'
code_patterns: ['sfImageRequest JSON struct', 'doGenerate() HTTP call', 'retry.Do()', 'resolveImageSize() model family mapping']
test_patterns: ['httptest.NewServer mock', 'testify assert/require', '//go:build integration tag']
---

# Tech-Spec: Character Reference Image-Edit Implementation

**Created:** 2026-03-15

## Overview

### Problem Statement

Selected character reference images are loaded and wired through the pipeline (`wireCharacterToImageSvc()` in 4 locations), but `SiliconFlowProvider.Edit()` is a stub returning `ErrNotSupported`. The service layer's Edit→Generate fallback works correctly, so currently all image generation falls through to text-to-image with prompt injection only. The actual source image bytes are never sent to the API.

### Solution

Implement `Edit()` using SiliconFlow's `/v1/images/generations` endpoint with `Qwen/Qwen-Image-Edit` model. This model accepts a source image via the `image` JSON field as a base64 data URI. The existing `doGenerate()` HTTP machinery is fully reusable — same endpoint, same response format.

### Scope

**In Scope:**
- Implement `SiliconFlowProvider.Edit()` with Qwen-Image-Edit model
- MIME type auto-detection for source image encoding (F1)
- Unit tests with mock HTTP server for Edit()
- Service layer integration test for Edit happy path (F6/F9)
- Update PoC integration test cleanup

**Out of Scope:**
- CharacterRef prompt composition — already implemented (`composeCharacterRefPrompt`)
- FLUX.1-Kontext model support (`input_image` field) — future enhancement
- DashScope direct API — future enhancement
- Pipeline runner changes — already wired correctly

## Context for Development

### Codebase Patterns

- `Generate()` builds `sfImageRequest` → JSON → `POST /v1/images/generations` → parses `sfImageResponse`
- `doGenerate()` is the internal HTTP call handler: marshal, request, error parsing, response decoding via `decodeImageData()`
- `decodeImageData()` handles base64 strings, data URIs, and HTTP URLs
- Retry: `retry.Do(ctx, maxRetries, baseDelay, fn)` — retries on `APIError.IsRetryable()` (429, 5xx, network)
- `resolveImageSize()` maps model family prefix → allowed sizes via `strings.Contains` on a map (non-deterministic iteration order — see F8 note). `"Qwen-Image"` family already registered with sizes `{"1664x928", "928x1664", "1328x1328", "1472x1140", "1140x1472", "1584x1056", "1056x1584"}`
- Service layer (`image_gen.go:82-101`): tries `Edit()` once as a probe outside retry loop, then on non-`ErrNotSupported` error enters retry loop for 3 more attempts (total 4 attempts — see F2 note). On `ErrNotSupported` falls back to `Generate()` with `genMethod = "fallback_t2i"`
- Constants are grouped in a single `const (...)` block at file top (lines 20-25)
- Existing imports in `siliconflow.go`: `bytes`, `context`, `encoding/base64`, `encoding/json`, `fmt`, `io`, `log/slog`, `net/http`, `strconv`, `strings`, `time` + internal packages `plugin`, `retry`

### Files to Reference

| File | Purpose |
| ---- | ------- |
| `internal/plugin/imagegen/siliconflow.go` | Provider — Edit() stub at line 342, doGenerate() at line 209, sfImageRequest at line 110, const block at line 20 |
| `internal/plugin/imagegen/siliconflow_test.go` | 15 existing unit tests — append Edit() tests |
| `internal/plugin/imagegen/siliconflow_edit_poc_test.go` | Integration PoC — currently blocked by stub, has stale comment (line 19) |
| `internal/plugin/imagegen/interface.go` | `ImageGen` interface, `EditOptions`, `CharacterRef`, `ErrNotSupported` |
| `internal/service/image_gen.go` | Edit fallback logic (lines 82-101) — probe-then-retry pattern |
| `internal/pipeline/runner.go` | `wireCharacterToImageSvc()` at line 1174 — no changes needed |
| `docs/silicon.image.api.spec.md` | SiliconFlow OpenAPI spec — Qwen-Image schema at line 233, upload_image schema at line 789 |

### Technical Decisions

1. **Same endpoint**: `Qwen/Qwen-Image-Edit` uses `/v1/images/generations` — NOT a separate `/images/edits` endpoint. The `image` field is what makes it an edit request.
2. **Default edit model**: Hard-coded `"Qwen/Qwen-Image-Edit"` as `defaultEditModel` constant, placed after `defaultImageHeight` in the existing `const` block (F13). Note: `Generate()` uses `p.model` (provider-level config) as its base, while `Edit()` uses `defaultEditModel` — this is intentional because the edit model is a different model family, not a variant of the generation model (F14).
3. **Reuse `doGenerate()`**: Edit builds an `sfImageRequest` with the `Image` field populated, then calls `doGenerate()`. No new HTTP code.
4. **MIME type auto-detection (F1)**: Use `http.DetectContentType(sourceImage)` to determine the actual image format instead of hardcoding `image/png`. The data URI becomes `"data:<detected-mime>;base64," + base64data`. This correctly handles JPEG, WebP, and PNG source images.
5. **Data URI format (F4)**: The API spec example shows `"data:image/png;base64, XXX"` (with space after comma). Implementation must match this format exactly: `"data:<mime>;base64, " + base64data` (note the trailing space before base64 content).
6. **Size resolution**: `resolveImageSize("Qwen/Qwen-Image-Edit", 1024, 576)` → matches `"Qwen-Image"` family → returns `"1664x928"` (16:9 default). No changes to `resolveImageSize()` needed. Note: `resolveImageSize()` uses `strings.Contains` on a map with non-deterministic iteration — currently safe because no model name matches multiple families, but fragile if new families are added (F8).
7. **No `NegativePrompt` in struct**: `EditOptions` does not expose `NegativePrompt`, so adding it to `sfImageRequest` would be a dead field. Only `Image` field is added.
8. **Empty sourceImage guard**: `Edit()` returns an early error if `sourceImage` is empty, preventing a malformed data URI from reaching the API.
9. **Character ref prompt in Edit (F5)**: `Edit()` does NOT call `composeCharacterRefPrompt()` — the source image itself provides the character reference. The visual descriptors are embedded in the image, not the text prompt. The prompt for Edit should describe the desired scene/composition, not the character appearance. This is a deliberate design choice: Edit uses image-based reference, Generate uses text-based reference.
10. **BatchSize omission (F12)**: Do NOT set `BatchSize: 1` explicitly. The field has `omitempty`, so omitting it lets the API use its default (1). This avoids masking API default behavior and is consistent with how optional fields should work.

## Implementation Plan

### Tasks

- [x] **Task 1**: Add `Image` field to `sfImageRequest`
  - File: `internal/plugin/imagegen/siliconflow.go` (line 110-118)
  - Action: Add one `omitempty` JSON field to the existing struct:
    ```go
    Image string `json:"image,omitempty"`
    ```
  - Notes: `NegativePrompt` is intentionally NOT added — `EditOptions` has no corresponding field, so it would be dead code.

- [x] **Task 2**: Implement `Edit()` method
  - File: `internal/plugin/imagegen/siliconflow.go` (line 340-344)
  - Action: Replace stub with implementation that:
    1. Returns early error if `sourceImage` is nil or empty (`len(sourceImage) == 0`)
    2. Defaults model to `defaultEditModel` constant (overridable via `opts.Model`)
    3. Resolves image size via `resolveImageSize()`
    4. Detects MIME type via `http.DetectContentType(sourceImage)` (F1)
    5. Encodes `sourceImage` as `"data:<mime>;base64, " + base64data` — note space after comma per API spec (F4)
    6. Builds `sfImageRequest` with `Image` field populated, `BatchSize` omitted (F12)
    7. Calls `doGenerate()` via `retry.Do()` (same pattern as `Generate()`)
    8. Logs duration with `time.Now()` / `time.Since(start)`, both success and error paths
  - Also: Add `defaultEditModel = "Qwen/Qwen-Image-Edit"` to existing `const (...)` block, after `defaultImageHeight` (F13)
  - Also: Add helper `detectImageMIME(data []byte) string` that wraps `http.DetectContentType` with fallback to `"image/png"`
  - Notes: Full reference code at bottom of spec. All required imports (`fmt`, `strings`, `strconv`, `encoding/base64`, `net/http`, `time`) are already present in the file (F3).

- [x] **Task 3**: Add Edit() unit tests
  - File: `internal/plugin/imagegen/siliconflow_test.go`
  - Action: Append 7 test functions after existing tests:
    1. `TestEdit_Success` — mock server validates `image` field present with correct data URI format (space after comma), `model` = `Qwen/Qwen-Image-Edit`, `image_size` = `"1664x928"` (Qwen default for 16:9), returns valid base64 image
    2. `TestEdit_CustomModel` — `EditOptions{Model: "custom"}` → request body has `model: "custom"` (F14 asymmetry documented)
    3. `TestEdit_ImageEncoding` — verify request body `image` field: (a) starts with `"data:image/png;base64, "` for PNG input, (b) starts with `"data:image/jpeg;base64, "` for JPEG input (F1 MIME detection)
    4. `TestEdit_ServerError_Retries` — 500 → 500 → 200 = success on 3rd attempt
    5. `TestEdit_EmptySourceImage` — `Edit(ctx, nil, ...)` and `Edit(ctx, []byte{}, ...)` both return error without hitting the server
    6. `TestEdit_EmptyResponse` — mock server returns 200 with `{"images": []}` → Edit returns error "no images returned" (F7)
    7. `TestEdit_BatchSizeOmitted` — verify request body does NOT contain `batch_size` field (or contains 0) (F12)
  - Notes: Follow exact pattern of existing `TestGenerate_*` tests. Use `httptest.NewServer`, decode `sfImageRequest` from request body, assert fields.

- [x] **Task 4**: Clean up PoC integration test
  - File: `internal/plugin/imagegen/siliconflow_edit_poc_test.go`
  - Action:
    1. Fix stale comment on line 19: change `SILICONFLOW_API_KEY` → `YTP_IMAGEGEN_API_KEY` (F10)
    2. Remove entire `ErrNotSupported` fallback block (lines 51-62) — this is SiliconFlow-specific test, and Edit() is now implemented. Keeping a dead `ErrNotSupported` branch is misleading (F15). Instead, let `editErr != nil` naturally fail the test.
    3. Verify the test compiles: `go build -tags=integration ./internal/plugin/imagegen/...`
  - Notes: The `ErrNotSupported` safety net concept was for multi-provider scenarios — not applicable to a provider-specific PoC test.

- [x] **Task 5**: Run `go test ./...` and `go vet ./...`
  - Action: Verify all existing + new tests pass, no vet warnings
  - Notes: Per project rules, both must pass before completion.

### Acceptance Criteria

- [ ] **AC1**: Given a valid source image ([]byte) and prompt, when `Edit()` is called, then it returns an `*ImageResult` with non-empty `ImageData`, `Format = "png"`, and correct `Width`/`Height`.

- [ ] **AC2**: Given a PNG source image, when `Edit()` builds the HTTP request, then the JSON body contains `"model": "Qwen/Qwen-Image-Edit"`, `"image": "data:image/png;base64, <valid-base64>"` (with space after comma), and `"image_size": "1664x928"` (Qwen 16:9 default).

- [ ] **AC3**: Given nil or empty `sourceImage`, when `Edit()` is called, then it returns an error immediately without making any HTTP request.

- [ ] **AC4**: Given a mock server returning HTTP 500 twice then 200, when `Edit()` is called, then it retries and returns a successful result on the 3rd attempt.

- [ ] **AC5**: Given `EditOptions{Model: "custom-model"}`, when `Edit()` is called, then the request body uses `"model": "custom-model"` instead of the default.

- [ ] **AC6**: Given a JPEG source image, when `Edit()` builds the request, then the data URI uses `"data:image/jpeg;base64, ..."` (auto-detected MIME type, not hardcoded PNG).

- [ ] **AC7**: Given a mock server returning 200 with empty `images` array, when `Edit()` is called, then it returns an error containing "no images returned".

- [ ] **AC8**: Given `Edit()` is called, when the request body is inspected, then `batch_size` is either absent or zero (not explicitly set to 1).

- [ ] **AC9**: Given `YTP_IMAGEGEN_API_KEY` is set, when `go test -tags=integration -run TestImageEditPoC -v ./internal/plugin/imagegen/...` is run, then the test executes against the real API and reports a score out of 10.

- [ ] **AC10**: `go test ./...` passes with zero failures. `go vet ./...` reports zero warnings.

## Additional Context

### Dependencies

- No new Go module dependencies
- All required imports already present in `siliconflow.go`: `encoding/base64`, `fmt`, `strings`, `strconv`, `net/http`, `time` (F3)
- External: SiliconFlow API with `Qwen/Qwen-Image-Edit` model availability

### Testing Strategy

- **Unit tests (Task 3)**: 7 new tests using `httptest.NewServer` to intercept requests and validate JSON body. No real API calls. Covers: success, custom model, MIME detection (PNG + JPEG), retry, empty source, empty response, batch_size omission.
- **Integration test (Task 4)**: Existing `siliconflow_edit_poc_test.go` with `//go:build integration` tag. Requires `YTP_IMAGEGEN_API_KEY` env var. Tests real API and reports quality score.
- **Run commands**:
  - Unit: `go test ./internal/plugin/imagegen/...`
  - Integration: `go test -tags=integration -run TestImageEditPoC -v ./internal/plugin/imagegen/...`
  - Full suite: `go test ./...`

### Notes

- **Risk: Qwen-Image-Edit API availability** — The model is listed in SiliconFlow's OpenAPI spec, but actual availability may vary. The PoC integration test (Task 4) validates this. If the model returns errors, the service layer falls back to Generate.
- **Base64 payload size (F11)** — Source image base64 encoding inflates size ~33% (e.g., 3MB PNG → ~4MB in JSON body). For production use, consider adding a source image size guard (e.g., reject images > 10MB before encoding). The current `http.Client.Timeout` via `pluginCfg.Timeout` provides a backstop, but a 20MB+ reference image could cause timeout with no actionable error. Recommendation: add `if len(sourceImage) > maxEditImageSize` early return in a follow-up.
- **Service layer probe-then-retry (F2)** — `image_gen.go:84-101` calls `Edit()` once outside the retry loop as a probe. If it fails with a transient error (not `ErrNotSupported`), it enters the retry loop for `MaxRetries` more attempts, totaling 4 calls. This is existing behavior and works correctly — the probe is needed to detect `ErrNotSupported` without wasting retry budget. However, this means a transient failure on the first Edit call gets 4 total attempts (1 probe + 3 retries), not 3. Acknowledge this in testing but do NOT change the service layer.
- **resolveImageSize fragility (F8)** — `resolveImageSize()` iterates a `map[string][]string` with `strings.Contains` matching. Map iteration order is non-deterministic in Go. Currently safe because no model string matches multiple family prefixes (e.g., `"Qwen-Image"` and `"FLUX.1-schnell"` are disjoint). If a model name ever contains multiple family substrings, results would be non-deterministic. Low risk for now, but worth noting for future model additions.
- **Edit vs Generate model asymmetry (F14)** — `Generate()` defaults to `p.model` (provider config), while `Edit()` defaults to `defaultEditModel` constant. This is intentional: the edit model (`Qwen/Qwen-Image-Edit`) is a different model family from the generation model (`FLUX.1-schnell`). The provider-level `model` config is for text-to-image; the edit model is a separate concern.
- **Character ref prompt not in Edit (F5)** — `Edit()` deliberately does NOT call `composeCharacterRefPrompt()`. The source image IS the character reference — visual descriptors are carried by the image pixels, not text. The edit prompt describes the desired scene/composition. If character descriptors were also prepended to the edit prompt, it would create redundancy (image + text both describing the character) and potentially confuse the model.
- **Future: FLUX.1-Kontext** — SiliconFlow also offers `FLUX.1-Kontext-max/pro/dev` models with `input_image` support. These could be an alternative or upgrade path for image editing. Out of scope for this spec.
- **No breaking changes** — All changes are additive. The `sfImageRequest` gets one new `omitempty` field (backward compatible). The `Edit()` method signature is unchanged.

## Review Notes
- Adversarial review completed
- Findings: 10 total, 3 fixed, 7 skipped (noise/documented design decisions)
- Resolution approach: auto-fix
- F4 (Low/Real): Added empty prompt validation → `APIError`
- F7 (Low/Real): Restricted `detectImageMIME` to PNG/JPEG/WebP via allowlist
- F8 (Medium/Real): Changed empty source image error from `fmt.Errorf` to `APIError` type

## Reference Code

### detectImageMIME helper

```go
// detectImageMIME returns the MIME type of image data, defaulting to "image/png".
func detectImageMIME(data []byte) string {
	if len(data) == 0 {
		return "image/png"
	}
	mime := http.DetectContentType(data)
	// http.DetectContentType may return types like "image/jpeg", "image/png", "image/webp"
	// For non-image types (e.g., "application/octet-stream"), default to PNG
	if !strings.HasPrefix(mime, "image/") {
		return "image/png"
	}
	return mime
}
```

### Edit() Implementation

```go
// Add to existing const block (line 20-25), after defaultImageHeight:
defaultEditModel = "Qwen/Qwen-Image-Edit"

// Replace Edit() stub (line 340-344):
func (p *SiliconFlowProvider) Edit(ctx context.Context, sourceImage []byte, prompt string, opts EditOptions) (*ImageResult, error) {
	if len(sourceImage) == 0 {
		return nil, fmt.Errorf("edit: source image is empty")
	}

	model := defaultEditModel
	if opts.Model != "" {
		model = opts.Model
	}

	width := opts.Width
	if width == 0 {
		width = defaultImageWidth
	}
	height := opts.Height
	if height == 0 {
		height = defaultImageHeight
	}

	imageSize := resolveImageSize(model, width, height)

	if parts := strings.SplitN(imageSize, "x", 2); len(parts) == 2 {
		if w, err := strconv.Atoi(parts[0]); err == nil {
			width = w
		}
		if h, err := strconv.Atoi(parts[1]); err == nil {
			height = h
		}
	}

	// Detect actual MIME type and encode as data URI (F1, F4)
	mime := detectImageMIME(sourceImage)
	b64Image := "data:" + mime + ";base64, " + base64.StdEncoding.EncodeToString(sourceImage)

	reqBody := sfImageRequest{
		Model:     model,
		Prompt:    prompt,
		ImageSize: imageSize,
		Image:     b64Image,
	}
	if opts.Seed != 0 {
		seed := opts.Seed
		reqBody.Seed = &seed
	}

	var result *ImageResult
	start := time.Now()

	err := retry.Do(ctx, p.pluginCfg.MaxRetries, p.pluginCfg.BaseDelay, func() error {
		var genErr error
		result, genErr = p.doGenerate(ctx, reqBody, width, height)
		return genErr
	})

	elapsed := time.Since(start)
	if err != nil {
		slog.Error("siliconflow image edit failed",
			"model", model,
			"duration_ms", elapsed.Milliseconds(),
			"err", err,
		)
		return nil, err
	}

	slog.Info("siliconflow image edited",
		"model", model,
		"width", result.Width,
		"height", result.Height,
		"format", result.Format,
		"size_bytes", len(result.ImageData),
		"duration_ms", elapsed.Milliseconds(),
	)

	return result, nil
}
```
