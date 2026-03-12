package cli

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

const validationTimeout = 10 * time.Second

// validationClient is a dedicated HTTP client for API key validation.
// It blocks redirects to prevent Bearer tokens from leaking to unexpected hosts.
var validationClient = &http.Client{
	Timeout: validationTimeout,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

// validateLLMKey validates an LLM provider API key.
func validateLLMKey(ctx context.Context, provider, apiKey string) error {
	switch provider {
	case "openai":
		return validateBearerToken(ctx, "https://api.openai.com/v1/models", apiKey, "LLM")
	default:
		return fmt.Errorf("init wizard: unknown LLM provider: %s", provider)
	}
}

// validateImageGenKey validates an image generation provider API key.
func validateImageGenKey(ctx context.Context, provider, apiKey string) error {
	switch provider {
	case "siliconflow":
		return validateBearerToken(ctx, "https://api.siliconflow.com/v1/models", apiKey, "ImageGen")
	default:
		return fmt.Errorf("init wizard: unknown ImageGen provider: %s", provider)
	}
}

// validateTTSKey validates a TTS provider API key.
func validateTTSKey(ctx context.Context, provider, apiKey string) error {
	switch provider {
	case "openai":
		return validateBearerToken(ctx, "https://api.openai.com/v1/models", apiKey, "TTS")
	case "google":
		// Google Cloud auth is complex; skip validation for now
		return nil
	case "edge":
		// Edge TTS doesn't need an API key
		return nil
	default:
		return fmt.Errorf("init wizard: unknown TTS provider: %s", provider)
	}
}

// validateBearerToken sends a GET request to url with a Bearer token and checks the response.
// It returns nil on 200, an actionable error on 401, and an error on other unexpected statuses.
func validateBearerToken(ctx context.Context, url, apiKey, serviceLabel string) error {
	ctx, cancel := context.WithTimeout(ctx, validationTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("init wizard: failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := validationClient.Do(req)
	if err != nil {
		return fmt.Errorf("init wizard: %s API key validation request failed: %w", serviceLabel, err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusUnauthorized:
		return fmt.Errorf(
			"init wizard: %s API key validation failed: 401 Unauthorized\n"+
				"  → Check your API key at the provider's dashboard\n"+
				"  → You can skip validation and set the key later via environment variable",
			serviceLabel,
		)
	default:
		return fmt.Errorf(
			"init wizard: %s API key validation returned unexpected status %d\n"+
				"  → The provider endpoint may be temporarily unavailable\n"+
				"  → You can skip validation and set the key later via environment variable",
			serviceLabel, resp.StatusCode,
		)
	}
}
