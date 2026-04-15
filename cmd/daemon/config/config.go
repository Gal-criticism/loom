package config

import (
    "os"
)

type Config struct {
    Runtime   string `yaml:"runtime"`
    BackendWS string `yaml:"backend_ws"`
    Listen    string `yaml:"listen"`
    LogLevel  string `yaml:"log_level"`
}

func Load(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return defaultConfig(), nil
    }

    var cfg Config
    if err := yaml.Unmarshal(data, &cfg); err != nil {
        return nil, err
    }

    return &cfg, nil
}

func defaultConfig() *Config {
    return &Config{
        Runtime:   "claude-code",
        BackendWS: "ws://localhost:3000/ws/daemon",
        Listen:    "localhost:3456",
        LogLevel:  "info",
    }
}
