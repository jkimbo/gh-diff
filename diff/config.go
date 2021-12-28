package diff

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	DefaultBranch string `yaml:"default_branch"`
}

var config *Config

func LoadConfig() (*Config, error) {
	if config == nil {
		config = &Config{}
		file, err := os.Open(filepath.Join(".stacked", "config.yaml"))
		if err != nil {
			return nil, err
		}
		defer file.Close()

		d := yaml.NewDecoder(file)
		if err := d.Decode(&config); err != nil {
			return nil, err
		}
	}

	return config, nil
}
