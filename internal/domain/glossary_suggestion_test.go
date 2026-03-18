package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCanSuggestionTransition_PendingToApproved(t *testing.T) {
	assert.True(t, CanSuggestionTransition(SuggestionPending, SuggestionApproved))
}

func TestCanSuggestionTransition_PendingToRejected(t *testing.T) {
	assert.True(t, CanSuggestionTransition(SuggestionPending, SuggestionRejected))
}

func TestCanSuggestionTransition_ApprovedIsTerminal(t *testing.T) {
	assert.False(t, CanSuggestionTransition(SuggestionApproved, SuggestionPending))
	assert.False(t, CanSuggestionTransition(SuggestionApproved, SuggestionRejected))
}

func TestCanSuggestionTransition_RejectedIsTerminal(t *testing.T) {
	assert.False(t, CanSuggestionTransition(SuggestionRejected, SuggestionPending))
	assert.False(t, CanSuggestionTransition(SuggestionRejected, SuggestionApproved))
}

func TestCanSuggestionTransition_UnknownStatus(t *testing.T) {
	assert.False(t, CanSuggestionTransition("unknown", SuggestionApproved))
	assert.False(t, CanSuggestionTransition("", SuggestionApproved))
}

func TestValidateGlossarySuggestion_Valid(t *testing.T) {
	err := ValidateGlossarySuggestion("proj-1", "SCP-173", "에스씨피")
	assert.NoError(t, err)
}

func TestValidateGlossarySuggestion_EmptyProjectID(t *testing.T) {
	err := ValidateGlossarySuggestion("", "SCP-173", "에스씨피")
	assert.Error(t, err)
	ve, ok := err.(*ValidationError)
	assert.True(t, ok)
	assert.Equal(t, "project_id", ve.Field)
}

func TestValidateGlossarySuggestion_EmptyTerm(t *testing.T) {
	err := ValidateGlossarySuggestion("proj-1", "", "에스씨피")
	assert.Error(t, err)
	ve, ok := err.(*ValidationError)
	assert.True(t, ok)
	assert.Equal(t, "term", ve.Field)
}

func TestValidateGlossarySuggestion_EmptyPronunciation(t *testing.T) {
	err := ValidateGlossarySuggestion("proj-1", "SCP-173", "")
	assert.Error(t, err)
	ve, ok := err.(*ValidationError)
	assert.True(t, ok)
	assert.Equal(t, "pronunciation", ve.Field)
}

func TestValidateGlossarySuggestion_FirstFieldFails(t *testing.T) {
	// All empty — should fail on project_id first
	err := ValidateGlossarySuggestion("", "", "")
	assert.Error(t, err)
	ve, ok := err.(*ValidationError)
	assert.True(t, ok)
	assert.Equal(t, "project_id", ve.Field)
}
