package cli

import (
	"fmt"
	"os"

	"github.com/sushistack/yt.pipe/internal/config"
	"github.com/sushistack/yt.pipe/internal/logging"
	"github.com/spf13/cobra"
)

var (
	cfgFile    string
	appConfig  *config.LoadResult
	rootCmd    = &cobra.Command{
		Use:   "yt-pipe",
		Short: "SCP YouTube content pipeline",
	}
)

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

// GetConfig returns the loaded config. Must be called after cobra.OnInitialize has run.
func GetConfig() *config.LoadResult {
	return appConfig
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file path")
	rootCmd.PersistentFlags().Bool("verbose", false, "enable verbose output")
	rootCmd.PersistentFlags().Bool("json-output", false, "output in JSON format")
}

func initConfig() {
	// Skip config loading for the init command — it creates the config file
	// and must work even when no config exists yet.
	if cmd, _, _ := rootCmd.Find(os.Args[1:]); cmd != nil && cmd.Name() == "init" {
		return
	}

	result, err := config.Load(cfgFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}
	appConfig = result

	// Apply --verbose flag: override log level to debug
	if verbose, _ := rootCmd.PersistentFlags().GetBool("verbose"); verbose {
		appConfig.Config.LogLevel = "debug"
		appConfig.Sources["log_level"] = "flag --verbose"
	}

	// Initialize structured logging
	logging.Setup(appConfig.Config.LogLevel, appConfig.Config.LogFormat)
}
