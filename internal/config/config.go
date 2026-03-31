package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

var (
	APIKey string
	Region string
)

var validRegions = map[string]string{
	"us":   "https://public-api.drata.com",
	"eu":   "https://public-api.eu.drata.com",
	"apac": "https://public-api.apac.drata.com",
}

func Init(region string) error {
	if path, err := ConfigPath(); err == nil {
		viper.SetConfigFile(path)
		_ = viper.ReadInConfig() // ignore "file not found" — config is optional
	}

	viper.SetEnvPrefix("DRATA")
	viper.AutomaticEnv()

	APIKey = viper.GetString("API_KEY")
	if APIKey == "" {
		return fmt.Errorf("DRATA_API_KEY is required (env var or ~/.config/drata-cli/config.yaml)")
	}

	if region != "" {
		Region = region
	} else {
		Region = viper.GetString("REGION")
		if Region == "" {
			Region = "us"
		}
	}

	if _, ok := validRegions[Region]; !ok {
		return fmt.Errorf("invalid region %q — valid: us, eu, apac", Region)
	}

	return nil
}

func BaseURL() string {
	return validRegions[Region]
}

// ConfigPath returns the path to the config file following XDG Base Directory spec.
// Uses $XDG_CONFIG_HOME if set, otherwise defaults to ~/.config.
func ConfigPath() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "drata-cli", "config.yaml"), nil
}

// WriteKey persists the API key to ~/.config/drata-cli/config.yaml.
func WriteKey(apiKey string) error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	// Read existing content to preserve other keys (e.g. region).
	viper.SetConfigFile(path)
	_ = viper.ReadInConfig()
	viper.Set("api_key", apiKey)
	if err := viper.WriteConfigAs(path); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}
