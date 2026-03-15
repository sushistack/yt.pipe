package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sushistack/yt.pipe/internal/domain"
)

func defaultQGConfig() QualityGateConfig {
	return QualityGateConfig{
		MaxAttempts:           3,
		FactCoverageThreshold: 80.0,
		MinSceneCount:         5,
		MaxSceneCount:         15,
		MinImmersionCount:     3,
	}
}

func makeScenario(scenes []domain.SceneScript) *domain.ScenarioOutput {
	return &domain.ScenarioOutput{
		SCPID:  "SCP-173",
		Title:  "Test Scenario",
		Scenes: scenes,
	}
}

func TestLayer1_HookCheck(t *testing.T) {
	cfg := defaultQGConfig()
	cfg.MinSceneCount = 1

	// Fail: starts with SCP-173
	scenario := makeScenario([]domain.SceneScript{
		{SceneNum: 1, Narration: "SCP-173은 유클리드 등급의 변칙 개체입니다.", Mood: "tense"},
		{SceneNum: 2, Narration: "당신은 이것을 보았습니다. 당신은 놀랐습니다. 당신은 도망쳤습니다.", Mood: "horror"},
		{SceneNum: 3, Narration: "격리실이 조용해졌습니다.", Mood: "suspense"},
		{SceneNum: 4, Narration: "아무도 없었습니다.", Mood: "tense"},
		{SceneNum: 5, Narration: "끝.", Mood: "awe"},
	})

	violations := RunLayer1(scenario, nil, cfg)
	found := false
	for _, v := range violations {
		if v.Check == "hook_pattern" {
			found = true
		}
	}
	assert.True(t, found, "should detect hook_pattern violation")

	// Pass: does not start with SCP-
	scenario.Scenes[0].Narration = "눈을 감지 마세요. 그 순간, 당신은 죽습니다."
	violations = RunLayer1(scenario, nil, cfg)
	for _, v := range violations {
		assert.NotEqual(t, "hook_pattern", v.Check, "should not flag hook_pattern")
	}
}

func TestLayer1_MoodVariation(t *testing.T) {
	cfg := defaultQGConfig()
	cfg.MinSceneCount = 1
	cfg.MinImmersionCount = 0

	// Fail: adjacent same mood
	scenario := makeScenario([]domain.SceneScript{
		{SceneNum: 1, Narration: "Hook sentence", Mood: "tense"},
		{SceneNum: 2, Narration: "Second scene", Mood: "horror"},
		{SceneNum: 3, Narration: "Third scene", Mood: "tense"},
		{SceneNum: 4, Narration: "Fourth scene", Mood: "tense"},
	})
	violations := RunLayer1(scenario, nil, cfg)
	found := false
	for _, v := range violations {
		if v.Check == "mood_variation" {
			found = true
			assert.Equal(t, 4, v.SceneNum) // scene 4 flagged
		}
	}
	assert.True(t, found, "should detect mood_variation violation")

	// Pass: alternating moods
	scenario.Scenes[3].Mood = "awe"
	violations = RunLayer1(scenario, nil, cfg)
	for _, v := range violations {
		assert.NotEqual(t, "mood_variation", v.Check)
	}
}

func TestLayer1_ImmersionCount(t *testing.T) {
	cfg := defaultQGConfig()
	cfg.MinSceneCount = 1

	// Fail: 0 "당신"
	scenario := makeScenario([]domain.SceneScript{
		{SceneNum: 1, Narration: "Hook sentence", Mood: "tense"},
	})
	violations := RunLayer1(scenario, nil, cfg)
	found := false
	for _, v := range violations {
		if v.Check == "immersion_count" {
			found = true
		}
	}
	assert.True(t, found, "should detect immersion_count violation")

	// Pass: 3+ "당신"
	scenario.Scenes[0].Narration = "당신이 격리실에 들어갔습니다. 당신은 두려웠습니다. 당신의 심장이 뛰었습니다."
	violations = RunLayer1(scenario, nil, cfg)
	for _, v := range violations {
		assert.NotEqual(t, "immersion_count", v.Check)
	}
}

func TestLayer1_SceneCount(t *testing.T) {
	cfg := defaultQGConfig()
	cfg.MinSceneCount = 5
	cfg.MaxSceneCount = 15
	cfg.MinImmersionCount = 0

	// Fail: too few scenes (3)
	scenario := makeScenario([]domain.SceneScript{
		{SceneNum: 1, Narration: "Hook", Mood: "tense"},
		{SceneNum: 2, Narration: "Two", Mood: "horror"},
		{SceneNum: 3, Narration: "Three", Mood: "awe"},
	})
	violations := RunLayer1(scenario, nil, cfg)
	found := false
	for _, v := range violations {
		if v.Check == "scene_count" {
			found = true
		}
	}
	assert.True(t, found, "should detect scene_count too few")

	// Pass: 8 scenes
	scenes := make([]domain.SceneScript, 8)
	moods := []string{"tense", "horror", "awe", "suspense", "tense", "horror", "awe", "suspense"}
	for i := range scenes {
		scenes[i] = domain.SceneScript{SceneNum: i + 1, Narration: "Scene", Mood: moods[i]}
	}
	scenario = makeScenario(scenes)
	violations = RunLayer1(scenario, nil, cfg)
	for _, v := range violations {
		assert.NotEqual(t, "scene_count", v.Check)
	}

	// Fail: too many scenes (20)
	scenes = make([]domain.SceneScript, 20)
	for i := range scenes {
		scenes[i] = domain.SceneScript{SceneNum: i + 1, Narration: "Scene", Mood: moods[i%len(moods)]}
	}
	scenario = makeScenario(scenes)
	violations = RunLayer1(scenario, nil, cfg)
	found = false
	for _, v := range violations {
		if v.Check == "scene_count" {
			found = true
		}
	}
	assert.True(t, found, "should detect scene_count too many")
}

func TestLayer1_FactCoverage(t *testing.T) {
	cfg := defaultQGConfig()
	cfg.MinSceneCount = 1
	cfg.MinImmersionCount = 0

	scenario := makeScenario([]domain.SceneScript{
		{SceneNum: 1, Narration: "Hook", Mood: "tense"},
	})

	// Fail: 60% < 80%
	review := &ReviewReport{CoveragePct: 60.0}
	violations := RunLayer1(scenario, review, cfg)
	found := false
	for _, v := range violations {
		if v.Check == "fact_coverage" {
			found = true
		}
	}
	assert.True(t, found, "should detect fact_coverage violation")

	// Pass: 85% >= 80%
	review.CoveragePct = 85.0
	violations = RunLayer1(scenario, review, cfg)
	for _, v := range violations {
		assert.NotEqual(t, "fact_coverage", v.Check)
	}
}

func TestLayer1_AllPass(t *testing.T) {
	cfg := defaultQGConfig()
	cfg.MinSceneCount = 5
	cfg.MaxSceneCount = 15
	cfg.MinImmersionCount = 3

	moods := []string{"tense", "horror", "awe", "suspense", "mystery"}
	scenes := make([]domain.SceneScript, 5)
	for i := range scenes {
		narration := "격리실 안의 이야기"
		if i == 0 {
			narration = "눈을 감는 순간, 당신은 죽습니다."
		}
		if i == 1 {
			narration = "당신이 격리실 문을 열었습니다."
		}
		if i == 2 {
			narration = "당신은 이 사실을 몰랐습니다."
		}
		scenes[i] = domain.SceneScript{SceneNum: i + 1, Narration: narration, Mood: moods[i]}
	}

	scenario := makeScenario(scenes)
	review := &ReviewReport{CoveragePct: 90.0}
	violations := RunLayer1(scenario, review, cfg)
	assert.Empty(t, violations, "all checks should pass")
}

func TestLayer2_ParseCriticVerdict(t *testing.T) {
	// Valid JSON
	validJSON := `{
		"verdict": "pass",
		"hook_effective": true,
		"retention_risk": "low",
		"ending_impact": "strong",
		"feedback": "좋은 시나리오입니다.",
		"scene_notes": [{"scene_num": 1, "issue": "none", "suggestion": "none"}]
	}`

	cleaned := extractJSONFromContent(validJSON)
	var verdict CriticVerdict
	err := json.Unmarshal([]byte(cleaned), &verdict)
	require.NoError(t, err)
	assert.Equal(t, "pass", verdict.Verdict)
	assert.True(t, verdict.HookEffective)
	assert.Equal(t, "low", verdict.RetentionRisk)
	assert.Equal(t, "strong", verdict.EndingImpact)
	assert.Len(t, verdict.SceneNotes, 1)

	// Malformed JSON
	malformed := `not json at all`
	err = json.Unmarshal([]byte(malformed), &verdict)
	assert.Error(t, err)
}

func TestBuildFeedbackString(t *testing.T) {
	violations := []QualityViolation{
		{Check: "hook_pattern", SceneNum: 1, Description: "Scene 1 starts with SCP-173"},
		{Check: "mood_variation", SceneNum: 4, Description: "Scenes 3-4 have same mood"},
	}
	critic := &CriticVerdict{
		Verdict:  "retry",
		Feedback: "Scene 1을 Shock Hook으로 교체하세요.",
		SceneNotes: []CriticSceneNote{
			{SceneNum: 1, Issue: "weak hook", Suggestion: "Use shock type"},
		},
	}

	feedback := BuildFeedbackString(violations, critic, 1, 3)
	assert.Contains(t, feedback, "QUALITY IMPROVEMENT REQUIRED")
	assert.Contains(t, feedback, "Attempt 2/3")
	assert.Contains(t, feedback, "hook_pattern")
	assert.Contains(t, feedback, "mood_variation")
	assert.Contains(t, feedback, "Content Director Feedback")
	assert.Contains(t, feedback, "Shock Hook")
	assert.Contains(t, feedback, "Scene-Specific Notes")
}

func TestBuildFeedbackString_ViolationsOnly(t *testing.T) {
	violations := []QualityViolation{
		{Check: "scene_count", SceneNum: 0, Description: "Scene count 3 < minimum 5"},
	}
	feedback := BuildFeedbackString(violations, nil, 1, 3)
	assert.Contains(t, feedback, "scene_count")
	assert.NotContains(t, feedback, "Content Director Feedback")
}

func TestBuildFeedbackString_Empty(t *testing.T) {
	feedback := BuildFeedbackString(nil, nil, 1, 3)
	assert.Empty(t, feedback)
}

func TestSelectBest(t *testing.T) {
	// nil current → candidate wins
	candidate := &writingAttempt{attempt: 1, violations: nil, criticVerdict: &CriticVerdict{Verdict: "pass"}}
	assert.Equal(t, candidate, selectBest(nil, candidate))

	// pass > accept_with_notes (higher score wins even if candidate is later)
	passAttempt := &writingAttempt{attempt: 1, criticVerdict: &CriticVerdict{Verdict: "pass"}}
	acceptAttempt := &writingAttempt{attempt: 2, criticVerdict: &CriticVerdict{Verdict: "accept_with_notes"}}
	// candidate (accept_with_notes=50) < current (pass=100), so current wins
	result := selectBest(passAttempt, acceptAttempt)
	assert.Equal(t, passAttempt, result)

	// accept_with_notes > retry
	retryAttempt := &writingAttempt{attempt: 3, criticVerdict: &CriticVerdict{Verdict: "retry"}}
	assert.Equal(t, acceptAttempt, selectBest(retryAttempt, acceptAttempt))

	// Among same verdict, later attempt (candidate) wins due to >= tie-break
	sameVerdict1 := &writingAttempt{attempt: 1, criticVerdict: &CriticVerdict{Verdict: "retry"}}
	sameVerdict2 := &writingAttempt{attempt: 2, criticVerdict: &CriticVerdict{Verdict: "retry"}}
	assert.Equal(t, sameVerdict2, selectBest(sameVerdict1, sameVerdict2))

	// Among same verdict, fewer violations wins
	fewViolations := &writingAttempt{
		attempt:       1,
		criticVerdict: &CriticVerdict{Verdict: "retry"},
		violations:    []QualityViolation{{Check: "one"}},
	}
	manyViolations := &writingAttempt{
		attempt:       2,
		criticVerdict: &CriticVerdict{Verdict: "retry"},
		violations:    []QualityViolation{{Check: "one"}, {Check: "two"}, {Check: "three"}},
	}
	assert.Equal(t, fewViolations, selectBest(manyViolations, fewViolations))
}

func TestSceneCountRange(t *testing.T) {
	min, max := sceneCountRange(5)
	assert.Equal(t, 4, min)
	assert.Equal(t, 7, max)

	min, max = sceneCountRange(10)
	assert.Equal(t, 7, min)
	assert.Equal(t, 12, max)

	min, max = sceneCountRange(15)
	assert.Equal(t, 10, min)
	assert.Equal(t, 16, max)

	min, max = sceneCountRange(20)
	assert.Equal(t, 7, min)
	assert.Equal(t, 12, max)
}
