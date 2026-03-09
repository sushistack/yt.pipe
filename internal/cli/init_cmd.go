package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/sushistack/yt.pipe/internal/service"
	"github.com/sushistack/yt.pipe/internal/store"
)

// lineReader wraps a bufio.Reader to implement io.Reader that returns data
// one line at a time. This prevents bufio.Scanner from over-reading when
// multiple scanners are created sequentially on the same underlying reader.
type lineReader struct {
	br  *bufio.Reader
	buf []byte // leftover from last ReadLine
}

func newLineReader(r io.Reader) *lineReader {
	return &lineReader{br: bufio.NewReader(r)}
}

func (lr *lineReader) Read(p []byte) (int, error) {
	if len(lr.buf) == 0 {
		line, err := lr.br.ReadBytes('\n')
		if len(line) == 0 && err != nil {
			return 0, err
		}
		lr.buf = line
	}
	n := copy(p, lr.buf)
	lr.buf = lr.buf[n:]
	return n, nil
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize pipeline configuration via interactive wizard",
	Long: `Run the interactive setup wizard to configure API keys, data paths,
and plugin preferences. Creates a global config file at $HOME/.yt-pipe/config.yaml.

Use --non-interactive for CI/scripted setup (reads from flags/env only).
Use --force to overwrite an existing configuration.`,
	SilenceErrors: true,
	RunE:          runInit,
}

func init() {
	initCmd.Flags().Bool("force", false, "overwrite existing configuration")
	initCmd.Flags().Bool("non-interactive", false, "run without interactive prompts (CI/scripted mode)")

	// Flags for non-interactive mode values
	initCmd.Flags().String("scp-data-path", "", "SCP data directory path")
	initCmd.Flags().String("workspace-path", "", "project workspace directory path")
	initCmd.Flags().String("llm-api-key", "", "LLM provider API key")
	initCmd.Flags().String("imagegen-api-key", "", "image generation provider API key")
	initCmd.Flags().String("tts-provider", "", "TTS provider (openai, google, edge)")
	initCmd.Flags().String("tts-api-key", "", "TTS provider API key")

	rootCmd.AddCommand(initCmd)
}

// wizardResult collects all values from the interactive wizard.
type wizardResult struct {
	SCPDataPath      string
	WorkspacePath    string
	LLMProvider      string
	LLMAPIKey        string
	ImageGenProvider string
	ImageGenAPIKey   string
	TTSProvider      string
	TTSAPIKey        string
	OutputProvider   string
}

func runInit(cmd *cobra.Command, args []string) error {
	force, _ := cmd.Flags().GetBool("force")
	nonInteractive, _ := cmd.Flags().GetBool("non-interactive")

	if nonInteractive {
		return runInitNonInteractive(cmd, force)
	}
	return runInitInteractive(cmd, force)
}

func runInitInteractive(_ *cobra.Command, force bool) error {
	return runWizard(os.Stdin, os.Stdout, force)
}

func runInitNonInteractive(cmd *cobra.Command, force bool) error {
	return runWizardNonInteractive(cmd, force)
}

// runWizard runs the interactive setup wizard.
func runWizard(r io.Reader, w io.Writer, force bool) error {
	// Wrap reader in lineReader to prevent bufio.Scanner from over-buffering
	// when multiple prompt calls create separate scanners on the same reader.
	r = newLineReader(r)

	// Determine config path
	configDir, err := defaultConfigDirFn()
	if err != nil {
		return fmt.Errorf("init wizard: %w", err)
	}
	configPath := filepath.Join(configDir, "config.yaml")

	// Check for existing config (unless --force)
	if !force {
		if _, statErr := os.Stat(configPath); statErr == nil {
			overwrite, confirmErr := promptConfirm(r, w, fmt.Sprintf("Config already exists at %s. Overwrite?", configPath), false)
			if confirmErr != nil {
				return confirmErr
			}
			if !overwrite {
				fmt.Fprintln(w, "Aborted. Use --force to overwrite.")
				return nil
			}
		}
	}

	// Step 1: Welcome
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "=== youtube.pipeline Setup Wizard ===")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "This wizard will help you configure your pipeline settings.")
	fmt.Fprintln(w, "Press Enter to accept defaults shown in [brackets].")
	fmt.Fprintln(w, "API keys can be skipped and set later via environment variables.")
	fmt.Fprintln(w, "")

	result := &wizardResult{}

	// Step 2: SCP data directory
	fmt.Fprintln(w, "--- Data Paths ---")
	result.SCPDataPath, err = promptString(r, w, "SCP data directory path", "")
	if err != nil {
		return err
	}
	if result.SCPDataPath != "" {
		if dirErr := validateOrCreateDir(r, w, result.SCPDataPath); dirErr != nil {
			return dirErr
		}
	}

	// Step 3: Workspace path
	result.WorkspacePath, err = promptString(r, w, "Project workspace path", "")
	if err != nil {
		return err
	}
	if result.WorkspacePath != "" {
		if dirErr := validateOrCreateDir(r, w, result.WorkspacePath); dirErr != nil {
			return dirErr
		}
	}

	fmt.Fprintln(w, "")

	// Step 4: LLM provider
	fmt.Fprintln(w, "--- LLM Plugin ---")
	result.LLMProvider = "openai"
	fmt.Fprintf(w, "LLM provider: %s (default)\n", result.LLMProvider)

	result.LLMAPIKey, err = promptSecret(r, w, "LLM API key (or Enter to skip)")
	if err != nil {
		return err
	}
	if result.LLMAPIKey != "" {
		skip, confirmErr := promptConfirm(r, w, "Skip API key validation?", true)
		if confirmErr != nil {
			return confirmErr
		}
		if !skip {
			if valErr := validateLLMKey(context.Background(), result.LLMProvider, result.LLMAPIKey); valErr != nil {
				fmt.Fprintf(w, "Warning: validation failed: %v\n", valErr)
				cont, contErr := promptConfirm(r, w, "Continue anyway?", true)
				if contErr != nil {
					return contErr
				}
				if !cont {
					return fmt.Errorf("init wizard: aborted due to API key validation failure")
				}
			} else {
				fmt.Fprintln(w, "API key validated successfully.")
			}
		}
	}

	fmt.Fprintln(w, "")

	// Step 5: ImageGen provider
	fmt.Fprintln(w, "--- Image Generation Plugin ---")
	result.ImageGenProvider = "siliconflow"
	fmt.Fprintf(w, "ImageGen provider: %s (default)\n", result.ImageGenProvider)

	result.ImageGenAPIKey, err = promptSecret(r, w, "ImageGen API key (or Enter to skip)")
	if err != nil {
		return err
	}
	if result.ImageGenAPIKey != "" {
		skip, confirmErr := promptConfirm(r, w, "Skip API key validation?", true)
		if confirmErr != nil {
			return confirmErr
		}
		if !skip {
			if valErr := validateImageGenKey(context.Background(), result.ImageGenProvider, result.ImageGenAPIKey); valErr != nil {
				fmt.Fprintf(w, "Warning: validation failed: %v\n", valErr)
				cont, contErr := promptConfirm(r, w, "Continue anyway?", true)
				if contErr != nil {
					return contErr
				}
				if !cont {
					return fmt.Errorf("init wizard: aborted due to API key validation failure")
				}
			} else {
				fmt.Fprintln(w, "API key validated successfully.")
			}
		}
	}

	fmt.Fprintln(w, "")

	// Step 6: TTS provider
	fmt.Fprintln(w, "--- TTS Plugin ---")
	result.TTSProvider, err = promptSelect(r, w, "TTS provider", []string{"openai", "google", "edge"}, 1)
	if err != nil {
		return err
	}

	if result.TTSProvider != "edge" {
		result.TTSAPIKey, err = promptSecret(r, w, "TTS API key (or Enter to skip)")
		if err != nil {
			return err
		}
		if result.TTSAPIKey != "" {
			skip, confirmErr := promptConfirm(r, w, "Skip API key validation?", true)
			if confirmErr != nil {
				return confirmErr
			}
			if !skip {
				if valErr := validateTTSKey(context.Background(), result.TTSProvider, result.TTSAPIKey); valErr != nil {
					fmt.Fprintf(w, "Warning: validation failed: %v\n", valErr)
					cont, contErr := promptConfirm(r, w, "Continue anyway?", true)
					if contErr != nil {
						return contErr
					}
					if !cont {
						return fmt.Errorf("init wizard: aborted due to API key validation failure")
					}
				} else {
					fmt.Fprintln(w, "API key validated successfully.")
				}
			}
		}
	} else {
		fmt.Fprintln(w, "Edge TTS selected - no API key required.")
	}

	fmt.Fprintln(w, "")

	// Step 7: Output provider (auto-selected)
	fmt.Fprintln(w, "--- Output Assembler ---")
	result.OutputProvider = "capcut"
	fmt.Fprintf(w, "Output provider: %s (default)\n", result.OutputProvider)

	fmt.Fprintln(w, "")

	// Generate config file
	if err := generateConfig(result, configDir); err != nil {
		return err
	}

	fmt.Fprintf(w, "Configuration written to %s\n", configPath)
	fmt.Fprintln(w, "")

	// Install default prompt templates
	if installed, installErr := installDefaultTemplates(configDir); installErr != nil {
		fmt.Fprintf(w, "Warning: failed to install default templates: %v\n", installErr)
	} else if installed > 0 {
		fmt.Fprintf(w, "Installed %d default prompt templates.\n", installed)
	} else {
		fmt.Fprintln(w, "Default prompt templates already installed, skipping.")
	}
	fmt.Fprintln(w, "")

	// Display summary
	displaySummary(w, result, configPath)

	return nil
}

// runWizardNonInteractive runs the non-interactive setup.
// Reads all values from flags and applies built-in defaults for unset providers.
// API key validation is skipped in non-interactive mode.
func runWizardNonInteractive(cmd *cobra.Command, force bool) error {
	// Determine config path
	configDir, err := defaultConfigDirFn()
	if err != nil {
		return fmt.Errorf("init wizard: %w", err)
	}
	configPath := filepath.Join(configDir, "config.yaml")

	// Check for existing config (unless --force)
	if !force {
		if _, statErr := os.Stat(configPath); statErr == nil {
			return fmt.Errorf("init wizard: config already exists at %s (use --force to overwrite)", configPath)
		}
	}

	// Read values from flags (fall back to defaults)
	scpDataPath, _ := cmd.Flags().GetString("scp-data-path")
	workspacePath, _ := cmd.Flags().GetString("workspace-path")
	llmAPIKey, _ := cmd.Flags().GetString("llm-api-key")
	imagegenAPIKey, _ := cmd.Flags().GetString("imagegen-api-key")
	ttsProvider, _ := cmd.Flags().GetString("tts-provider")
	ttsAPIKey, _ := cmd.Flags().GetString("tts-api-key")

	// Apply defaults for providers
	if ttsProvider == "" {
		ttsProvider = "openai"
	}

	result := &wizardResult{
		SCPDataPath:      scpDataPath,
		WorkspacePath:    workspacePath,
		LLMProvider:      "openai",
		LLMAPIKey:        llmAPIKey,
		ImageGenProvider: "siliconflow",
		ImageGenAPIKey:   imagegenAPIKey,
		TTSProvider:      ttsProvider,
		TTSAPIKey:        ttsAPIKey,
		OutputProvider:   "capcut",
	}

	// Create directories if paths provided
	if result.SCPDataPath != "" {
		if mkErr := os.MkdirAll(result.SCPDataPath, 0755); mkErr != nil {
			return fmt.Errorf("init wizard: creating SCP data directory: %w", mkErr)
		}
	}
	if result.WorkspacePath != "" {
		if mkErr := os.MkdirAll(result.WorkspacePath, 0755); mkErr != nil {
			return fmt.Errorf("init wizard: creating workspace directory: %w", mkErr)
		}
	}

	// Generate config (no API validation in non-interactive mode)
	if err := generateConfig(result, configDir); err != nil {
		return err
	}

	w := cmd.OutOrStdout()

	// Install default prompt templates
	if installed, installErr := installDefaultTemplates(configDir); installErr != nil {
		fmt.Fprintf(w, "Warning: failed to install default templates: %v\n", installErr)
	} else if installed > 0 {
		fmt.Fprintf(w, "Installed %d default prompt templates.\n", installed)
	} else {
		fmt.Fprintln(w, "Default prompt templates already installed, skipping.")
	}

	// Display summary via cobra's writer (testable)
	displaySummary(w, result, configPath)

	return nil
}

// validateOrCreateDir checks if a directory exists. If not, prompts the user
// to create it and creates it with os.MkdirAll.
func validateOrCreateDir(r io.Reader, w io.Writer, path string) error {
	info, err := os.Stat(path)
	if err == nil {
		if !info.IsDir() {
			return fmt.Errorf("init wizard: %s exists but is not a directory", path)
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("init wizard: %w", err)
	}

	create, confirmErr := promptConfirm(r, w, fmt.Sprintf("Directory %s does not exist. Create it?", path), true)
	if confirmErr != nil {
		return confirmErr
	}
	if !create {
		return fmt.Errorf("init wizard: directory %s does not exist", path)
	}

	if mkErr := os.MkdirAll(path, 0755); mkErr != nil {
		return fmt.Errorf("init wizard: creating directory: %w", mkErr)
	}
	fmt.Fprintf(w, "Created %s\n", path)
	return nil
}

// defaultConfigDirFn returns the default config directory ($HOME/.yt-pipe).
// It is a variable so tests can override it.
var defaultConfigDirFn = func() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("determining home directory: %w", err)
	}
	return filepath.Join(home, ".yt-pipe"), nil
}

// generateConfig creates the config directory and writes config.yaml.
// API keys are NEVER stored as plaintext values - only as comments with
// environment variable instructions.
func generateConfig(result *wizardResult, configDir string) error {
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("init wizard: creating config directory: %w", err)
	}

	configPath := filepath.Join(configDir, "config.yaml")
	f, err := os.OpenFile(configPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("init wizard: creating config file: %w", err)
	}
	defer f.Close()

	date := time.Now().Format("2006-01-02")

	var b strings.Builder

	// Header
	fmt.Fprintf(&b, "# youtube.pipeline configuration\n")
	fmt.Fprintf(&b, "# Generated by yt-pipe init on %s\n", date)
	fmt.Fprintf(&b, "#\n")
	fmt.Fprintf(&b, "# Configuration Priority (highest to lowest):\n")
	fmt.Fprintf(&b, "#   1. CLI flags (--config, --verbose, --json-output)\n")
	fmt.Fprintf(&b, "#   2. Environment variables (YTP_ prefix)\n")
	fmt.Fprintf(&b, "#   3. Project config (./config.yaml in working directory)\n")
	fmt.Fprintf(&b, "#   4. Global config ($HOME/.yt-pipe/config.yaml) <- this file\n")
	fmt.Fprintf(&b, "#   5. Built-in defaults\n")
	fmt.Fprintf(&b, "\n")

	// Data Paths
	fmt.Fprintf(&b, "# Data Paths\n")
	fmt.Fprintf(&b, "scp_data_path: %q\n", result.SCPDataPath)
	fmt.Fprintf(&b, "workspace_path: %q\n", result.WorkspacePath)
	fmt.Fprintf(&b, "\n")

	// LLM Plugin
	fmt.Fprintf(&b, "# LLM Plugin\n")
	fmt.Fprintf(&b, "llm:\n")
	fmt.Fprintf(&b, "  provider: %q\n", result.LLMProvider)
	fmt.Fprintf(&b, "  # API key - set via environment variable:\n")
	fmt.Fprintf(&b, "  #   export YTP_LLM_API_KEY=\"your-key-here\"\n")
	fmt.Fprintf(&b, "  # api_key: \"\"\n")
	fmt.Fprintf(&b, "\n")

	// Image Generation Plugin
	fmt.Fprintf(&b, "# Image Generation Plugin\n")
	fmt.Fprintf(&b, "imagegen:\n")
	fmt.Fprintf(&b, "  provider: %q\n", result.ImageGenProvider)
	fmt.Fprintf(&b, "  # API key - set via environment variable:\n")
	fmt.Fprintf(&b, "  #   export YTP_IMAGEGEN_API_KEY=\"your-key-here\"\n")
	fmt.Fprintf(&b, "  # api_key: \"\"\n")
	fmt.Fprintf(&b, "\n")

	// TTS Plugin
	fmt.Fprintf(&b, "# TTS Plugin\n")
	fmt.Fprintf(&b, "tts:\n")
	fmt.Fprintf(&b, "  provider: %q\n", result.TTSProvider)
	if result.TTSProvider != "edge" {
		fmt.Fprintf(&b, "  # API key - set via environment variable:\n")
		fmt.Fprintf(&b, "  #   export YTP_TTS_API_KEY=\"your-key-here\"\n")
		fmt.Fprintf(&b, "  # api_key: \"\"\n")
	} else {
		fmt.Fprintf(&b, "  # Edge TTS requires no API key\n")
	}
	fmt.Fprintf(&b, "\n")

	// Output Assembler
	fmt.Fprintf(&b, "# Output Assembler\n")
	fmt.Fprintf(&b, "output:\n")
	fmt.Fprintf(&b, "  provider: %q\n", result.OutputProvider)

	if _, err := f.WriteString(b.String()); err != nil {
		return fmt.Errorf("init wizard: writing config file: %w", err)
	}

	return nil
}

// displaySummary prints a formatted summary of the wizard results and next steps.
func displaySummary(w io.Writer, result *wizardResult, configPath string) {
	fmt.Fprintln(w, "=== Setup Summary ===")
	fmt.Fprintln(w, "")

	fmt.Fprintf(w, "  Config file:     %s\n", configPath)
	fmt.Fprintf(w, "  SCP data path:   %s\n", displayPath(result.SCPDataPath))
	fmt.Fprintf(w, "  Workspace path:  %s\n", displayPath(result.WorkspacePath))
	fmt.Fprintf(w, "  LLM provider:    %s\n", result.LLMProvider)
	fmt.Fprintf(w, "  LLM API key:     %s\n", maskKey(result.LLMAPIKey))
	fmt.Fprintf(w, "  ImageGen:        %s\n", result.ImageGenProvider)
	fmt.Fprintf(w, "  ImageGen key:    %s\n", maskKey(result.ImageGenAPIKey))
	fmt.Fprintf(w, "  TTS provider:    %s\n", result.TTSProvider)
	fmt.Fprintf(w, "  TTS API key:     %s\n", maskKey(result.TTSAPIKey))
	fmt.Fprintf(w, "  Output:          %s\n", result.OutputProvider)

	fmt.Fprintln(w, "")

	// Show environment variable instructions (never print actual key values)
	fmt.Fprintln(w, "--- Set API Keys ---")
	fmt.Fprintln(w, "Add these to your shell profile (~/.bashrc or ~/.zshrc):")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  export YTP_LLM_API_KEY=\"your-openai-key\"")
	fmt.Fprintln(w, "  export YTP_IMAGEGEN_API_KEY=\"your-siliconflow-key\"")
	if result.TTSProvider != "edge" {
		fmt.Fprintf(w, "  export YTP_TTS_API_KEY=\"your-%s-key\"\n", result.TTSProvider)
	}
	fmt.Fprintln(w, "")

	fmt.Fprintln(w, "--- Next Steps ---")
	fmt.Fprintln(w, "  1. Source your shell profile or export the variables above")
	fmt.Fprintln(w, "  2. Run: yt-pipe config validate")
	fmt.Fprintln(w, "  3. Run: yt-pipe config show")
	fmt.Fprintln(w, "")
}

// maskKey masks an API key for display. Shows first 4 chars + "..." or "(not set)".
func maskKey(key string) string {
	if key == "" {
		return "(not set)"
	}
	if len(key) <= 4 {
		return "****"
	}
	return key[:4] + "****"
}

// installDefaultTemplates opens the database in configDir and installs default templates.
func installDefaultTemplates(configDir string) (int, error) {
	dbPath := filepath.Join(configDir, "yt-pipe.db")
	db, err := store.New(dbPath)
	if err != nil {
		return 0, fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	svc := service.NewTemplateService(db)
	return svc.InstallDefaults()
}

// displayPath returns the path or "(not set)" if empty.
func displayPath(p string) string {
	if p == "" {
		return "(not set)"
	}
	return p
}
