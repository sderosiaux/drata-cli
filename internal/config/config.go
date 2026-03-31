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
	home, err := os.UserHomeDir()
	if err == nil {
		viper.AddConfigPath(filepath.Join(home, ".config", "drata-cli"))
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		// ignore "file not found" errors — config is optional
		_ = viper.ReadInConfig()
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
