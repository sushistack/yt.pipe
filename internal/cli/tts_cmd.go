package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
)

var ttsRegisterVoiceCmd = &cobra.Command{
	Use:   "register-voice",
	Short: "Register a voice clone with DashScope",
	Long:  "Upload an audio sample to DashScope for voice cloning and receive a VoiceID",
	RunE:  runTTSRegisterVoice,
}

func init() {
	ttsRegisterVoiceCmd.Flags().String("audio", "", "Path to audio sample file (WAV format)")
	ttsRegisterVoiceCmd.Flags().String("name", "", "Name for the cloned voice")
	_ = ttsRegisterVoiceCmd.MarkFlagRequired("audio")
	_ = ttsRegisterVoiceCmd.MarkFlagRequired("name")
	ttsCmd.AddCommand(ttsRegisterVoiceCmd)
}

// dsVoiceRegisterRequest is the DashScope voice registration request.
type dsVoiceRegisterRequest struct {
	Model string                 `json:"model"`
	Input dsVoiceRegisterInput   `json:"input"`
}

type dsVoiceRegisterInput struct {
	AudioURL string `json:"audio_url"`
	Name     string `json:"name"`
}

type dsVoiceRegisterResponse struct {
	RequestID string `json:"request_id"`
	Output    struct {
		VoiceID string `json:"voice_id"`
	} `json:"output"`
}

func runTTSRegisterVoice(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true

	audioPath, _ := cmd.Flags().GetString("audio")
	voiceName, _ := cmd.Flags().GetString("name")

	cfg := GetConfig()
	if cfg == nil {
		return fmt.Errorf("tts register-voice: configuration not loaded")
	}
	c := cfg.Config

	if c.TTS.APIKey == "" {
		return fmt.Errorf("tts register-voice: tts.api_key not configured")
	}

	// Read audio file
	audioData, err := os.ReadFile(audioPath)
	if err != nil {
		return fmt.Errorf("tts register-voice: read audio file: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Registering voice clone %q from %s (%d bytes)...\n", voiceName, audioPath, len(audioData))

	// NOTE: DashScope voice registration requires uploading audio to a URL first,
	// then calling the registration API. This implementation uses the local path
	// as a placeholder. In production, integrate with DashScope's file upload API.
	endpoint := c.TTS.Endpoint
	if endpoint == "" {
		endpoint = "https://dashscope.aliyuncs.com"
	}

	reqBody := dsVoiceRegisterRequest{
		Model: "cosyvoice-clone-v1",
		Input: dsVoiceRegisterInput{
			AudioURL: audioPath, // In production, this would be an uploaded URL
			Name:     voiceName,
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	url := endpoint + "/api/v1/services/aigc/voice/register"
	req, err := http.NewRequestWithContext(cmd.Context(), http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.TTS.APIKey)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("tts register-voice: API call failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("tts register-voice: API error (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	var dsResp dsVoiceRegisterResponse
	if err := json.Unmarshal(respBody, &dsResp); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	voiceID := "cosyvoice-clone-" + dsResp.Output.VoiceID

	fmt.Fprintf(cmd.OutOrStdout(), "\nVoice clone registered successfully!\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  Voice ID: %s\n", voiceID)
	fmt.Fprintf(cmd.OutOrStdout(), "\nTo use this voice, add to your config.yaml:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  tts:\n    voice: %q\n", voiceID)

	_ = audioData // audioData would be used in full implementation with file upload

	return nil
}
