package modsimpro

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path"
)

type Config struct {
	Serials []Serial `json:"serial"`
}

type Serial struct {
	Url      string  `json:"url"`
	Timeout  int     `json:"timeout"`
	Speed    int     `json:"speed"`
	DataBits int     `json:"data_bits"`
	Parity   int     `json:"parity"`
	StopBits int     `json:"stop_bits"`
	Slaves   []Slave `json:"slaves"`
}

type Slave struct {
	Address int    `json:"address,omitempty"`
	Type    string `json:"type"`
}

func LoadConfig(configPath string) (Config, error) {
	if !exists(path.Join(configPath, "config.json")) {
		return Config{}, fmt.Errorf("configuration file not found: %s", path.Join(configPath, "config.json"))
	}

	bb, err := os.ReadFile(path.Join(configPath, "config.json"))
	if err != nil {
		return Config{}, fmt.Errorf("error reading file: %w", err)
	}
	var config Config
	if err := json.NewDecoder(bytes.NewReader(bb)).Decode(&config); err != nil {
		return Config{}, fmt.Errorf("error decoding file: %w", err)
	}
	return config, nil
}

func exists(filePath string) bool {
	_, err := os.Stat(filePath)
	return err == nil || !os.IsNotExist(err)
}
