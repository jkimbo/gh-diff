package diff

import (
	"os"
	"os/exec"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type config struct {
	DefaultBranch string `yaml:"default_branch"`
}

func initConfig(rootPath string) *config {
	// Get base branch name
	gitCmd := exec.Command("gh", "repo", "view", "--json=defaultBranchRef", "--jq=.defaultBranchRef.name")
	defaultBranch := mustCommand(gitCmd, true, false)

	config := &config{
		DefaultBranch: defaultBranch,
	}

	d, err := yaml.Marshal(&config)
	check(err)
	err = os.WriteFile(filepath.Join(rootPath, ".diff", "config.yaml"), d, 0644)
	check(err)

	return config
}

func loadConfig() (*config, error) {
	config := &config{}
	file, err := os.Open(filepath.Join(".diff", "config.yaml"))
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
