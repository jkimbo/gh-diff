package diff

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type config struct {
	defaultBranch string `yaml:"default_branch"`
}

func loadConfig() (*config, error) {
	config := &config{}
	file, err := os.Open(filepath.Join(".stacked", "config.yaml"))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	d := yaml.NewDecoder(file)
	if err := d.Decode(&config); err != nil {
		return nil, err
	}

	return config, nil
}
