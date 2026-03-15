package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sushistack/yt.pipe/internal/plugin/tts"
	"github.com/sushistack/yt.pipe/internal/store"
)

var ttsRegisterVoiceCmd = &cobra.Command{
	Use:   "register-voice",
	Short: "Register a voice clone with DashScope",
	Long:  "Upload an audio sample to DashScope for voice cloning via qwen-voice-enrollment API and receive a VoiceID",
	RunE:  runTTSRegisterVoice,
}

var ttsTestVoiceCmd = &cobra.Command{
	Use:   "test-voice",
	Short: "Test a voice sample for cloning quality",
	Long:  "Enroll a voice sample, synthesize test text, and optionally save the voice ID for production use",
	RunE:  runTTSTestVoice,
}

func init() {
	ttsRegisterVoiceCmd.Flags().String("audio", "", "Path to audio sample file")
	ttsRegisterVoiceCmd.Flags().String("name", "", "Preferred name for the cloned voice")
	ttsRegisterVoiceCmd.Flags().String("project", "", "Project ID to cache voice ID (optional)")
	_ = ttsRegisterVoiceCmd.MarkFlagRequired("audio")
	_ = ttsRegisterVoiceCmd.MarkFlagRequired("name")
	ttsCmd.AddCommand(ttsRegisterVoiceCmd)

	ttsTestVoiceCmd.Flags().String("sample", "", "Path to voice sample audio file")
	ttsTestVoiceCmd.Flags().String("text", "", "Text to synthesize with the cloned voice")
	ttsTestVoiceCmd.Flags().String("save-voice-id", "", "Project ID to save voice ID for production use")
	_ = ttsTestVoiceCmd.MarkFlagRequired("sample")
	_ = ttsTestVoiceCmd.MarkFlagRequired("text")
	ttsCmd.AddCommand(ttsTestVoiceCmd)
}

func runTTSRegisterVoice(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true

	audioPath, _ := cmd.Flags().GetString("audio")
	voiceName, _ := cmd.Flags().GetString("name")
	projectID, _ := cmd.Flags().GetString("project")

	provider, cleanup, err := createTTSProvider(cmd)
	if err != nil {
		return err
	}
	defer cleanup()

	fmt.Fprintf(cmd.OutOrStdout(), "Registering voice clone %q from %s...\n", voiceName, audioPath)

	voiceID, err := provider.CreateVoice(cmd.Context(), audioPath, voiceName)
	if err != nil {
		return fmt.Errorf("tts register-voice: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "\nVoice clone registered successfully!\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  Voice ID: %s\n", voiceID)
	fmt.Fprintf(cmd.OutOrStdout(), "\nTo use this voice, set in config.yaml:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  tts:\n    voice: %q\n", voiceID)

	// Optionally cache to project DB
	if projectID != "" {
		if err := cacheVoiceToProject(projectID, voiceID, audioPath); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to cache voice ID: %v\n", err)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "  Voice ID cached for project %s\n", projectID)
		}
	}

	return nil
}

func runTTSTestVoice(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true

	samplePath, _ := cmd.Flags().GetString("sample")
	text, _ := cmd.Flags().GetString("text")
	saveVoiceID, _ := cmd.Flags().GetString("save-voice-id")

	provider, cleanup, err := createTTSProvider(cmd)
	if err != nil {
		return err
	}
	defer cleanup()

	// Step 1: Enroll voice
	preferredName := strings.TrimSuffix(filepath.Base(samplePath), filepath.Ext(samplePath))

	fmt.Fprintf(cmd.OutOrStdout(), "Enrolling voice from %s...\n", samplePath)

	voiceID, err := provider.CreateVoice(cmd.Context(), samplePath, preferredName)
	if err != nil {
		return fmt.Errorf("tts test-voice: enrollment failed: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "  Voice ID: %s\n", voiceID)

	// Step 2: Synthesize test text
	fmt.Fprintf(cmd.OutOrStdout(), "Synthesizing test text...\n")

	result, err := provider.Synthesize(cmd.Context(), text, voiceID, nil)
	if err != nil {
		return fmt.Errorf("tts test-voice: synthesis failed: %w", err)
	}

	// Step 3: Save output
	sampleBasename := strings.TrimSuffix(filepath.Base(samplePath), filepath.Ext(samplePath))
	outputPath := fmt.Sprintf("tts-test-%s.wav", sampleBasename)

	if err := os.WriteFile(outputPath, result.AudioData, 0o644); err != nil {
		return fmt.Errorf("tts test-voice: save output: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "\nTest voice output saved: %s\n", outputPath)
	fmt.Fprintf(cmd.OutOrStdout(), "  Duration: %.1f seconds\n", result.DurationSec)
	fmt.Fprintf(cmd.OutOrStdout(), "  Audio size: %d bytes\n", len(result.AudioData))
	fmt.Fprintf(cmd.OutOrStdout(), "\nVoice sample quality guidelines:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  - Recommended: 10+ seconds of clean, single-speaker audio\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  - Avoid background noise, music, or multiple speakers\n")

	// Step 4: Optionally save voice ID
	if saveVoiceID != "" {
		if err := cacheVoiceToProject(saveVoiceID, voiceID, samplePath); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to save voice ID: %v\n", err)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "  Voice ID saved as %q for production use\n", saveVoiceID)
		}
	}

	return nil
}

// createTTSProvider creates a DashScope TTS provider from config.
func createTTSProvider(cmd *cobra.Command) (*tts.DashScopeProvider, func(), error) {
	cfg := GetConfig()
	if cfg == nil {
		return nil, func() {}, fmt.Errorf("configuration not loaded")
	}
	c := cfg.Config

	if c.TTS.APIKey == "" {
		return nil, func() {}, fmt.Errorf("tts.api_key not configured")
	}

	provider, err := tts.NewDashScopeProvider(tts.DashScopeConfig{
		Endpoint:   c.TTS.Endpoint,
		APIKey:     c.TTS.APIKey,
		Model:      c.TTS.Model,
		CloneModel: c.TTS.Clone.Model,
		Format:     c.TTS.Format,
		Voice:      c.TTS.Voice,
		Language:   c.TTS.Language,
	})
	if err != nil {
		return nil, func() {}, fmt.Errorf("create TTS provider: %w", err)
	}

	return provider, func() {}, nil
}

// cacheVoiceToProject stores a voice ID in the project DB.
func cacheVoiceToProject(projectID, voiceID, samplePath string) error {
	cfg := GetConfig()
	if cfg == nil {
		return fmt.Errorf("configuration not loaded")
	}

	db, err := store.New(cfg.Config.DBPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	return db.CacheVoice(projectID, voiceID, samplePath)
}
