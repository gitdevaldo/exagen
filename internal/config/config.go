package config

import (
	"encoding/json"
	"os"
)

// Config holds the application configuration.
type Config struct {
	Proxy         string `json:"proxy"`
	OutputFile    string `json:"output_file"`
	DefaultDomain string `json:"default_domain"`
}

const (
	DefaultProxy          = ""
	DefaultOutputFile     = "results.txt"
	DefaultConfigFilename = "config.json"
	DefaultDomainValue    = ""
)

// Load reads the config from a JSON file and applies environment variable overrides.
func Load(path string) (*Config, error) {
	cfg := &Config{
		Proxy:         DefaultProxy,
		OutputFile:    DefaultOutputFile,
		DefaultDomain: DefaultDomainValue,
	}

	data, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	if proxy := os.Getenv("PROXY"); proxy != "" {
		cfg.Proxy = proxy
	}

	return cfg, nil
}
