package mocks_test

import (
	"context"
	"testing"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestMock_LLMGenerateScenario(t *testing.T) {
	m := mocks.NewMockLLM(t)

	expected := &domain.ScenarioOutput{
		SCPID: "SCP-173",
		Title: "Test Scenario",
		Scenes: []domain.SceneScript{
			{SceneNum: 1, Narration: "Test narration"},
		},
	}

	m.EXPECT().GenerateScenario(
		mock.Anything,
		"SCP-173",
		"main text",
		[]domain.FactTag{{Key: "k1", Content: "c1"}},
		map[string]string{"key": "value"},
	).Return(expected, nil)

	result, err := m.GenerateScenario(
		context.Background(),
		"SCP-173",
		"main text",
		[]domain.FactTag{{Key: "k1", Content: "c1"}},
		map[string]string{"key": "value"},
	)

	assert.NoError(t, err)
	assert.Equal(t, expected, result)
	m.AssertExpectations(t)
}
