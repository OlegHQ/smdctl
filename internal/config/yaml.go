package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// envVarPattern matches ${VAR} or $VAR patterns
var envVarPattern = regexp.MustCompile(`\$\{([^}]+)\}|\$([a-zA-Z_][a-zA-Z0-9_]*)`)

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

	Tasks []TaskConfig `yaml:"tasks"`
}

type TaskConfig struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	Command     string            `yaml:"command"`
	Args        []string          `yaml:"args"`
	WorkDir     string            `yaml:"workdir"`
	Environment map[string]string `yaml:"environment"`
	Schedule    TaskSchedule      `yaml:"schedule"`
}

type TaskSchedule struct {
	OnCalendar        string `yaml:"on_calendar"`
	OnBootSec         string `yaml:"on_boot_sec"`
	OnStartupSec      string `yaml:"on_startup_sec"`
	OnUnitActiveSec   string `yaml:"on_unit_active_sec"`
	OnUnitInactiveSec string `yaml:"on_unit_inactive_sec"`

	Persistent         *bool  `yaml:"persistent"`
	RandomizedDelaySec string `yaml:"randomized_delay_sec"`
	AccuracySec        string `yaml:"accuracy_sec"`
}

// LoadFromFile loads a YAML configuration from a file
func LoadFromFile(path string) (*ServiceConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	cfg, err := ParseYAML(data)
	if err != nil {
		return nil, err
	}

	// Expand ${VAR} patterns from environment
	ExpandEnvVars(cfg)

	return cfg, nil
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

	// Task defaults
	for i := range cfg.Tasks {
		if cfg.Tasks[i].WorkDir == "" {
			cfg.Tasks[i].WorkDir = cfg.WorkDir
		}

		// Inherit + override environment
		if cfg.Tasks[i].Environment == nil {
			cfg.Tasks[i].Environment = map[string]string{}
		}
		for k, v := range cfg.Environment {
			if _, ok := cfg.Tasks[i].Environment[k]; !ok {
				cfg.Tasks[i].Environment[k] = v
			}
		}

		// Default schedule options
		if cfg.Tasks[i].Schedule.Persistent == nil {
			defaultPersistent := true
			cfg.Tasks[i].Schedule.Persistent = &defaultPersistent
		}
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

// expandEnvVar expands ${VAR} or $VAR patterns in a string.
// Returns the expanded string and any variable names that were not found.
func expandEnvVar(s string) (string, []string) {
	var missing []string

	result := envVarPattern.ReplaceAllStringFunc(s, func(match string) string {
		// Extract variable name from ${VAR} or $VAR
		var varName string
		if strings.HasPrefix(match, "${") {
			varName = match[2 : len(match)-1] // ${VAR} -> VAR
		} else {
			varName = match[1:] // $VAR -> VAR
		}

		value, exists := os.LookupEnv(varName)
		if !exists {
			missing = append(missing, varName)
			return "" // Leave empty if not found
		}
		return value
	})

	return result, missing
}

// ExpandEnvVars expands ${VAR} patterns in all string fields of the config.
// Missing variables are logged as warnings and replaced with empty strings.
func ExpandEnvVars(cfg *ServiceConfig) {
	var allMissing []string

	// Expand top-level string fields
	cfg.Name, _ = expandEnvVar(cfg.Name)
	cfg.Description, _ = expandEnvVar(cfg.Description)
	cfg.Command, _ = expandEnvVar(cfg.Command)
	cfg.WorkDir, _ = expandEnvVar(cfg.WorkDir)
	cfg.User, _ = expandEnvVar(cfg.User)
	cfg.Restart, _ = expandEnvVar(cfg.Restart)
	cfg.KillMode, _ = expandEnvVar(cfg.KillMode)
	cfg.ProtectSystem, _ = expandEnvVar(cfg.ProtectSystem)

	// Expand args
	for i, arg := range cfg.Args {
		cfg.Args[i], _ = expandEnvVar(arg)
	}

	// Expand after/wants
	for i, v := range cfg.After {
		cfg.After[i], _ = expandEnvVar(v)
	}
	for i, v := range cfg.Wants {
		cfg.Wants[i], _ = expandEnvVar(v)
	}

	// Expand environment variables
	for k, v := range cfg.Environment {
		expanded, missing := expandEnvVar(v)
		cfg.Environment[k] = expanded
		allMissing = append(allMissing, missing...)
	}

	// Expand tasks
	for i := range cfg.Tasks {
		cfg.Tasks[i].Name, _ = expandEnvVar(cfg.Tasks[i].Name)
		cfg.Tasks[i].Description, _ = expandEnvVar(cfg.Tasks[i].Description)
		cfg.Tasks[i].Command, _ = expandEnvVar(cfg.Tasks[i].Command)
		cfg.Tasks[i].WorkDir, _ = expandEnvVar(cfg.Tasks[i].WorkDir)

		for j, arg := range cfg.Tasks[i].Args {
			cfg.Tasks[i].Args[j], _ = expandEnvVar(arg)
		}

		for k, v := range cfg.Tasks[i].Environment {
			expanded, missing := expandEnvVar(v)
			cfg.Tasks[i].Environment[k] = expanded
			allMissing = append(allMissing, missing...)
		}
	}

	// Warn about missing variables
	if len(allMissing) > 0 {
		// Deduplicate
		seen := make(map[string]bool)
		var unique []string
		for _, v := range allMissing {
			if !seen[v] {
				seen[v] = true
				unique = append(unique, v)
			}
		}
		fmt.Printf("Warning: environment variables not set (using empty value): %s\n", strings.Join(unique, ", "))
	}
}
