// Package llm defines the interface for language model plugins.
package llm

import (
	"context"

	"github.com/jay/youtube-pipeline/internal/domain"
)

//go:generate go run github.com/vektra/mockery/v2@latest --name=LLM --output=../../../internal/mocks --outpkg=mocks

// LLM defines the interface for language model plugins.
type LLM interface {
	// GenerateScenario generates a complete scenario from SCP data.
	GenerateScenario(ctx context.Context, scpID string, mainText string, facts []domain.FactTag, metadata map[string]string) (*domain.ScenarioOutput, error)

	// RegenerateSection regenerates a single scene's script based on instruction.
	RegenerateSection(ctx context.Context, scenario *domain.ScenarioOutput, sceneNum int, instruction string) (*domain.SceneScript, error)
}
