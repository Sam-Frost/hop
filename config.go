package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Server represents a single SSH target.
type Server struct {
	Name string `json:"name"`
	IP   string `json:"ip"`
	User string `json:"user"`
	Pem  string `json:"pem"` // if empty, DefaultPem is used
}

// Config is the persisted application state.
type Config struct {
	DefaultPem string   `json:"default_pem"`
	Servers    []Server `json:"servers"`
}

func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	appDir := filepath.Join(dir, "ssh-manager")
	if err := os.MkdirAll(appDir, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(appDir, "config.json"), nil
}

func LoadConfig() (Config, error) {
	var cfg Config
	path, err := configPath()
	if err != nil {
		return cfg, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func SaveConfig(cfg Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// PemFor returns the pem path a server should connect with,
// falling back to the configured default when the server has none set.
func (c Config) PemFor(s Server) string {
	if s.Pem != "" {
		return s.Pem
	}
	return c.DefaultPem
}
