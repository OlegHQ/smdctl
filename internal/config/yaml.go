package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ServiceConfig represents the YAML configuration for a service
type ServiceConfig struct {
	Name            string            `yaml:"name"`
	Description     string            `yaml:"description"`
	Command         string            `yaml:"command"`
	Args            []string          `yaml:"args"`
	WorkDir         string            `yaml:"workdir"`
	User            string            `yaml:"user"`
	Environment     map[string]string `yaml:"environment"`
	Restart         string            `yaml:"restart"`
	SystemMode      bool              `yaml:"system_mode"`
	TimeoutStart    int               `yaml:"timeout_start"`
	TimeoutStop     int               `yaml:"timeout_stop"`
	KillMode        string            `yaml:"kill_mode"`
	After           []string          `yaml:"after"`
	Wants           []string          `yaml:"wants"`
	PrivateTmp      bool              `yaml:"private_tmp"`
	ProtectSystem   string            `yaml:"protect_system"`
	NoNewPrivileges bool              `yaml:"no_new_privileges"`
	LimitNOFILE     int               `yaml:"limit_nofile"`
	TasksMax        int               `yaml:"tasks_max"`
}

// LoadFromFile loads a YAML configuration from a file
func LoadFromFile(path string) (*ServiceConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	return ParseYAML(data)
}

// ParseYAML parses YAML data into a ServiceConfig
func ParseYAML(data []byte) (*ServiceConfig, error) {
	var cfg ServiceConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}

	// Set defaults
	if cfg.Restart == "" {
		cfg.Restart = "always"
	}
	if cfg.TimeoutStart == 0 {
		cfg.TimeoutStart = 90
	}
	if cfg.TimeoutStop == 0 {
		cfg.TimeoutStop = 30
	}
	if cfg.KillMode == "" {
		cfg.KillMode = "control-group"
	}
	if cfg.WorkDir == "" {
		cfg.WorkDir = "/"
	}

	return &cfg, nil
}

// FindConfigFile looks for smdctl.yml in the current directory
func FindConfigFile() (string, error) {
	candidates := []string{
		"smdctl.yml",
		"smdctl.yaml",
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("config file not found (looked for: smdctl.yml, smdctl.yaml)")
}
