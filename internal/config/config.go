// Package config provides helpers for loading the application settings from YAML.
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is the root configuration object.
type Config struct {
	LogLevel  string           `yaml:"log_level"`
	Device    string           `yaml:"device"`
	DeviceID  string           `yaml:"device_id"`
	Providers []ProviderConfig `yaml:"providers"`
}

// ProviderConfig represents a single power meter source, such as Tasmota or a Mock.
type ProviderConfig struct {
	Type         string  `yaml:"type"`
	IP           string  `yaml:"ip"`
	User         string  `yaml:"user"`
	Password     string  `yaml:"password"`
	Status       string  `yaml:"status"`
	Payload      string  `yaml:"payload"`
	Label        string  `yaml:"label"`
	LabelIn      string  `yaml:"label_in"`
	LabelOut     string  `yaml:"label_out"`
	Calculate    bool    `yaml:"calculate"`
	Throttle     float64 `yaml:"throttle"`
	StaleTimeout float64 `yaml:"stale_timeout"`
	Power        float64 `yaml:"power"`
	Broker       string  `yaml:"broker"`
	Port         int     `yaml:"port"`
	Topic        string  `yaml:"topic"`
	JsonPath     string  `yaml:"json_path"`
	JsonPathIn   string  `yaml:"json_path_in"`
	JsonPathOut  string  `yaml:"json_path_out"`
}

// Load reads and parses the YAML configuration file at the given path.
// If the file does not exist, it returns an empty configuration.
func Load(path string) (Config, error) {
	var cfg Config

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	defer f.Close()

	decoder := yaml.NewDecoder(f)
	err = decoder.Decode(&cfg)
	if err != nil {
		return cfg, fmt.Errorf("failed to decode config: %w", err)
	}

	return cfg, nil
}
