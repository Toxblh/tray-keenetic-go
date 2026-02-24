package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"

	"github.com/99designs/keyring"
)

// RouterConfig holds a configured router entry.
type RouterConfig struct {
	Name      string   `json:"name"`
	Address   string   `json:"address"`
	Login     string   `json:"login"`
	NetworkIP string   `json:"network_ip,omitempty"`
	KeenDNS   []string `json:"keendns_urls,omitempty"`
}

var globalRing keyring.Keyring

func init() {
	ring, err := keyring.Open(keyring.Config{
		ServiceName: "keenetic-tray",
		AllowedBackends: []keyring.BackendType{
			keyring.WinCredBackend,
			keyring.KeychainBackend,
			keyring.SecretServiceBackend,
			keyring.KWalletBackend,
		},
	})
	if err == nil {
		globalRing = ring
	}
}

func configDir() string {
	switch runtime.GOOS {
	case "darwin":
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Application Support", "RouterManager")
	case "windows":
		return filepath.Join(os.Getenv("APPDATA"), "RouterManager")
	default:
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".router_manager")
	}
}

func configPath() string {
	dir := configDir()
	_ = os.MkdirAll(dir, 0o755)
	return filepath.Join(dir, "routers.json")
}

func loadRouters() []RouterConfig {
	data, err := os.ReadFile(configPath())
	if err != nil {
		return nil
	}
	var routers []RouterConfig
	if err := json.Unmarshal(data, &routers); err != nil {
		return nil
	}
	return routers
}

func saveRouters(routers []RouterConfig) error {
	data, err := json.MarshalIndent(routers, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(), data, 0o644)
}

func getPassword(name string) string {
	if globalRing == nil {
		return ""
	}
	item, err := globalRing.Get(name)
	if err != nil {
		return ""
	}
	return string(item.Data)
}

func setPassword(name, password string) {
	if globalRing == nil {
		return
	}
	_ = globalRing.Set(keyring.Item{
		Key:         name,
		Data:        []byte(password),
		Label:       "Keenetic Tray - " + name,
		Description: "Router password",
	})
}

func deletePassword(name string) {
	if globalRing == nil {
		return
	}
	_ = globalRing.Remove(name)
}
