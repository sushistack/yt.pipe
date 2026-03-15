package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/plugin/llm"
)

// QualityGateConfig configures the quality gate thresholds.
type QualityGateConfig struct {
	MaxAttempts           int
	FactCoverageThreshold float64
	MinSceneCount         int
	MaxSceneCount         int
	MinImmersionCount     int
}

// QualityViolation is a single structural check failure from Layer 1.
type QualityViolation struct {
	Check       string
	SceneNum    int
	Description string
}

// CriticVerdict is the parsed JSON output from the Critic Agent (Layer 2).
type CriticVerdict struct {
	Verdict       string            `json:"verdict"`
	HookEffective bool              `json:"hook_effective"`
	RetentionRisk string            `json:"retention_risk"`
	EndingImpact  string            `json:"ending_impact"`
	Feedback      string            `json:"feedback"`
	SceneNotes    []CriticSceneNote `json:"scene_notes"`
}

// CriticSceneNote is per-scene feedback from the Critic Agent.
type CriticSceneNote struct {
	SceneNum   int    `json:"scene_num"`
	Issue      string `json:"issue"`
	Suggestion string `json:"suggestion"`
}

var hookPatternRe = regexp.MustCompile(`^SCP-\d+`)

// RunLayer1 performs code-based structural validation on a scenario.
// Returns a slice of violations; empty means Layer 1 pass.
func RunLayer1(scenario *domain.ScenarioOutput, reviewReport *ReviewReport, cfg QualityGateConfig) []QualityViolation {
	var violations []QualityViolation

	// Hook check: Scene 1 narration must not start with "SCP-XXX" pattern
	if len(scenario.Scenes) > 0 {
		narration := strings.TrimSpace(scenario.Scenes[0].Narration)
		if hookPatternRe.MatchString(narration) {
			violations = append(violations, QualityViolation{
				Check:       "hook_pattern",
				SceneNum:    1,
				Description: fmt.Sprintf("Scene 1 narration starts with SCP classification pattern: %q", truncateQG(narration, 50)),
			})
		}
	}

	// Mood variation: no two adjacent scenes with same mood
	for i := 1; i < len(scenario.Scenes); i++ {
		prev := scenario.Scenes[i-1].Mood
		curr := scenario.Scenes[i].Mood
		if prev != "" && curr != "" && strings.EqualFold(prev, curr) {
			violations = append(violations, QualityViolation{
				Check:       "mood_variation",
				SceneNum:    scenario.Scenes[i].SceneNum,
				Description: fmt.Sprintf("Scenes %d-%d have same mood %q", scenario.Scenes[i-1].SceneNum, scenario.Scenes[i].SceneNum, curr),
			})
		}
	}

	// Immersion count: "당신" occurrences across all narrations
	immersionCount := 0
	for _, scene := range scenario.Scenes {
		immersionCount += strings.Count(scene.Narration, "당신")
	}
	if immersionCount < cfg.MinImmersionCount {
		violations = append(violations, QualityViolation{
			Check:       "immersion_count",
			SceneNum:    0,
			Description: fmt.Sprintf("Immersion device count %d < minimum %d (\"당신\" occurrences)", immersionCount, cfg.MinImmersionCount),
		})
	}

	// Scene count: within configured range
	sceneCount := len(scenario.Scenes)
	if sceneCount < cfg.MinSceneCount {
		violations = append(violations, QualityViolation{
			Check:       "scene_count",
			SceneNum:    0,
			Description: fmt.Sprintf("Scene count %d < minimum %d", sceneCount, cfg.MinSceneCount),
		})
	}
	if sceneCount > cfg.MaxSceneCount {
		violations = append(violations, QualityViolation{
			Check:       "scene_count",
			SceneNum:    0,
			Description: fmt.Sprintf("Scene count %d > maximum %d", sceneCount, cfg.MaxSceneCount),
		})
	}

	// Fact coverage: check review report if available
	if reviewReport != nil && cfg.FactCoverageThreshold > 0 {
		if reviewReport.CoveragePct < cfg.FactCoverageThreshold {
			violations = append(violations, QualityViolation{
				Check:       "fact_coverage",
				SceneNum:    0,
				Description: fmt.Sprintf("Fact coverage %.1f%% < threshold %.1f%%", reviewReport.CoveragePct, cfg.FactCoverageThreshold),
			})
		}
	}

	return violations
}

// RunLayer2 invokes the Critic Agent via LLM to evaluate format guide compliance.
// Returns nil verdict (not error) on parse failures for graceful degradation.
func RunLayer2(ctx context.Context, l llm.LLM, scenario *domain.ScenarioOutput, formatGuide, criticTemplate string) (*CriticVerdict, error) {
	scenarioJSON, err := json.MarshalIndent(scenario, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("quality gate: marshal scenario: %w", err)
	}

	// Replace {format_guide} FIRST (before scenario injection) to prevent
	// LLM-generated content containing "{format_guide}" from being replaced.
	prompt := strings.ReplaceAll(criticTemplate, "{format_guide}", formatGuide)
	prompt = strings.ReplaceAll(prompt, "{scenario_json}", string(scenarioJSON))

	result, err := l.Complete(ctx, []llm.Message{
		{Role: "user", Content: prompt},
	}, llm.CompletionOptions{})
	if err != nil {
		return nil, fmt.Errorf("quality gate: critic agent LLM call: %w", err)
	}

	cleaned := extractJSONFromContent(result.Content)
	var verdict CriticVerdict
	if err := json.Unmarshal([]byte(cleaned), &verdict); err != nil {
		slog.Warn("quality gate: could not parse critic verdict, treating as pass", "err", err)
		return nil, nil
	}

	// Normalize verdict to lowercase for case-insensitive matching
	verdict.Verdict = strings.ToLower(verdict.Verdict)

	return &verdict, nil
}

// BuildFeedbackString combines Layer 1 violations and Critic feedback into a structured
// feedback string for injection into the Writing prompt's {quality_feedback} variable.
func BuildFeedbackString(violations []QualityViolation, criticVerdict *CriticVerdict, attempt, maxAttempts int) string {
	if len(violations) == 0 && criticVerdict == nil {
		return ""
	}

	nextAttempt := attempt + 1
	if nextAttempt > maxAttempts {
		nextAttempt = maxAttempts
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("\n## ⚠️ QUALITY IMPROVEMENT REQUIRED (Attempt %d/%d)\n\n", nextAttempt, maxAttempts))
	b.WriteString("Your previous scenario was rejected. Fix these specific issues:\n\n")

	if len(violations) > 0 {
		b.WriteString("### Code Validation Failures:\n")
		for _, v := range violations {
			if v.SceneNum > 0 {
				b.WriteString(fmt.Sprintf("- Scene %d [%s]: %s\n", v.SceneNum, v.Check, v.Description))
			} else {
				b.WriteString(fmt.Sprintf("- [%s]: %s\n", v.Check, v.Description))
			}
		}
		b.WriteString("\n")
	}

	if criticVerdict != nil && criticVerdict.Feedback != "" {
		b.WriteString("### Content Director Feedback:\n")
		b.WriteString(criticVerdict.Feedback)
		b.WriteString("\n\n")

		if len(criticVerdict.SceneNotes) > 0 {
			b.WriteString("### Scene-Specific Notes:\n")
			for _, note := range criticVerdict.SceneNotes {
				b.WriteString(fmt.Sprintf("- Scene %d: %s → %s\n", note.SceneNum, note.Issue, note.Suggestion))
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("DO NOT repeat the same mistakes. Address each issue above.\n")
	return b.String()
}

// writingAttempt tracks a single writing+review attempt for best-attempt selection.
type writingAttempt struct {
	scenario      *domain.ScenarioOutput
	reviewReport  *ReviewReport
	writingStage  *StageResult
	reviewStage   *StageResult
	violations    []QualityViolation
	criticVerdict *CriticVerdict
	attempt       int
}

// selectBest returns the better of two attempts based on verdict priority and violation count.
// On equal score, prefers the later attempt (has latest feedback incorporated).
func selectBest(current, candidate *writingAttempt) *writingAttempt {
	if current == nil {
		return candidate
	}
	if candidate == nil {
		return current
	}
	if attemptScore(candidate) >= attemptScore(current) {
		return candidate
	}
	return current
}

// attemptScore assigns a numeric score for comparison.
// Higher is better: verdict priority + fewer violations.
func attemptScore(a *writingAttempt) int {
	score := 0

	// Verdict priority: pass=100, accept_with_notes=50, retry (critic reviewed)=20, no critic=15
	if a.criticVerdict != nil {
		switch a.criticVerdict.Verdict {
		case "pass":
			score += 100
		case "accept_with_notes":
			score += 50
		case "retry":
			score += 20
		default:
			score += 20 // unknown verdict — treat as retry
		}
	} else {
		// No critic run — Layer 1 passed or critic not configured
		if len(a.violations) == 0 {
			score += 15
		}
	}

	// Fewer violations = better (subtract violation count)
	score -= len(a.violations) * 5

	return score
}

// sceneCountRange returns min/max scene count based on target duration.
func sceneCountRange(targetDurationMin int) (int, int) {
	switch {
	case targetDurationMin <= 5:
		return 4, 7
	case targetDurationMin <= 10:
		return 7, 12
	case targetDurationMin <= 15:
		return 10, 16
	default:
		return 7, 12
	}
}

// truncateQG truncates a string by rune count for quality gate violation descriptions.
// Uses rune-based slicing to avoid cutting multi-byte Korean characters.
func truncateQG(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "..."
}
