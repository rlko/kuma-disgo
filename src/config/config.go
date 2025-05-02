package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Discord struct {
		Token         string `yaml:"token"`
		ApplicationID string `yaml:"application_id"`
		GuildID       string `yaml:"guild_id"`
	} `yaml:"discord"`
	UptimeKuma struct {
		APIKey  string `yaml:"api_key"`
		BaseURL string `yaml:"base_url"`
	} `yaml:"uptime_kuma"`
	Sections []Section `yaml:"sections"`
	UpdateInterval time.Duration `yaml:"update_interval"` // Update interval in seconds
}

type Section struct {
	Name     string    `yaml:"name"`     // Section display name
	Services []Service `yaml:"services"` // Services in this section
}

type Service struct {
	Name        string `yaml:"name"`         // Must match monitor_name in Uptime Kuma metrics
	DisplayName string `yaml:"display_name,omitempty"` // Optional name shown in Discord embed
	Description string `yaml:"description"`  // Optional description
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	// Set default update interval if not specified
	if config.UpdateInterval == 0 {
		config.UpdateInterval = 60 * time.Second
	}

	return &config, nil
} 