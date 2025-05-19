package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/rlko/kuma-disgo/src/config"
	"github.com/rlko/kuma-disgo/src/db"
	"github.com/rlko/kuma-disgo/src/discord"
	"github.com/rlko/kuma-disgo/src/kuma"
)

var (
	configPath string
)

var rootCmd = &cobra.Command{
	Use:   "kuma-disgo",
	Short: "Discord bot for monitoring Uptime Kuma services",
	Long: `A Discord bot that monitors and displays the status of services
monitored by Uptime Kuma. It provides real-time status updates and
allows server owners to view service statuses through Discord commands.`,
	RunE: run,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "Path to config file")
}

func run(cmd *cobra.Command, args []string) error {
	// Determine config file path with fallbacks
	var cfgPath string
	if configPath != "" {
		cfgPath = configPath
	} else {
		// Try ~/.config/kuma-disgo/config.yaml
		homeDir, err := os.UserHomeDir()
		if err == nil {
			defaultConfig := filepath.Join(homeDir, ".config", "kuma-disgo", "config.yaml")
			if _, err := os.Stat(defaultConfig); err == nil {
				cfgPath = defaultConfig
			}
		}

		// If not found, try current directory
		if cfgPath == "" {
			execPath, err := os.Executable()
			if err == nil {
				cfgPath = filepath.Join(filepath.Dir(execPath), "config.yaml")
			}
		}
	}

	if cfgPath == "" {
		return fmt.Errorf("no config file found. Please specify a config file using -c flag or place it in ~/.config/kuma-disgo/config.yaml or the same directory as the executable")
	}

	log.Printf("Using config file: %s", cfgPath)

	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	kumaClient := kuma.NewClient(cfg.UptimeKuma.BaseURL, cfg.UptimeKuma.APIKey)

	// Initialize StatusStore
	statusDBPath := filepath.Join(filepath.Dir(cfgPath), "status.db")
	statusStore, err := db.NewStatusStore(statusDBPath)
	if err != nil {
		return fmt.Errorf("failed to initialize status store: %w", err)
	}

	bot, err := discord.NewBot(cfg.Discord.Token, kumaClient, cfg, statusStore)
	if err != nil {
		return fmt.Errorf("failed to create bot: %w", err)
	}

	if err = bot.Start(context.Background()); err != nil {
		return fmt.Errorf("failed to start bot: %w", err)
	}

	log.Println("Bot is running. Press CTRL-C to exit.")
	select {}
}
