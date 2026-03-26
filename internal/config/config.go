package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	LLM    LLMConfig    `json:"llm"`
	Agent  AgentConfig  `json:"agent"`
	Memory MemoryConfig `json:"memory"`
}

type LLMConfig struct {
	Backends map[string]BackendConfig `json:"backends"`
	Routing  RoutingConfig            `json:"routing"`
}

type BackendConfig struct {
	Endpoint  string `json:"endpoint,omitempty"`
	Model     string `json:"model"`
	APIKeyEnv string `json:"api_key_env,omitempty"`
}

type RoutingConfig struct {
	Default              string   `json:"default"`
	Fallback             string   `json:"fallback"`
	ForceCloudFor        []string `json:"force_cloud_for"`
	HealthCheckIntervalS int      `json:"health_check_interval_sec"`
}

type AgentConfig struct {
	MaxIterations int `json:"max_iterations"`
}

type MemoryConfig struct {
	ConversationHistoryLimit int `json:"conversation_history_limit"`
}

func Default() *Config {
	return &Config{
		LLM: LLMConfig{
			Backends: map[string]BackendConfig{
				"ollama": {
					Endpoint: "http://localhost:11434",
					Model:    "qwen3-coder:14b",
				},
				"claude": {
					Model:    "claude-sonnet-4-6-20250514",
					APIKeyEnv: "ANTHROPIC_API_KEY",
				},
				"gemini": {
					Model:    "gemini-3.1-flash-lite-preview",
					APIKeyEnv: "GEMINI_API_KEY",
				},
			},
			Routing: RoutingConfig{
				Default:              "ollama",
				Fallback:             "claude",
				ForceCloudFor:        []string{"skill_creation"},
				HealthCheckIntervalS: 30,
			},
		},
		Agent: AgentConfig{
			MaxIterations: 10,
		},
		Memory: MemoryConfig{
			ConversationHistoryLimit: 50,
		},
	}
}

func DataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".goemon"), nil
}

func ConfigPath() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func Load() (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Default(), nil
		}
		return nil, err
	}

	cfg := Default()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) Save(path string) error {
	data, err := json.MarshalIndent(c, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
