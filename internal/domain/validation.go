package domain

// ValidationResult represents the quality assessment of a generated image.
type ValidationResult struct {
	Score            int      // 0-100 overall weighted score
	PromptMatch      int      // 0-100 prompt consistency
	CharacterMatch   int      // 0-100 character consistency, -1 if no character
	TechnicalScore   int      // 0-100 technical quality (no artifacts/distortions)
	Reasons          []string // explanation for each sub-score
	ShouldRegenerate bool     // true if Score < threshold
}

// CalculateScore computes the overall Score as a weighted average of sub-scores.
// When CharacterMatch == -1 (no character), weight is redistributed:
//
//	With character:    prompt_match*0.5 + character_match*0.3 + technical_score*0.2
//	Without character: prompt_match*0.7 + technical_score*0.3
func (v *ValidationResult) CalculateScore() {
	if v.CharacterMatch == -1 {
		v.Score = int(float64(v.PromptMatch)*0.7 + float64(v.TechnicalScore)*0.3)
	} else {
		v.Score = int(float64(v.PromptMatch)*0.5 + float64(v.CharacterMatch)*0.3 + float64(v.TechnicalScore)*0.2)
	}
}

// Evaluate calculates the score and sets ShouldRegenerate based on threshold.
func (v *ValidationResult) Evaluate(threshold int) {
	v.CalculateScore()
	v.ShouldRegenerate = v.Score < threshold
}
