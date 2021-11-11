package cmd

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3" // so that sqlx works with sqlite
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type Config struct {
	DefaultBranch string `yaml:"default_branch"`
}

var config *Config

func loadConfig() (*Config, error) {
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

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialise stacked",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Initialise stacked")

		newpath := filepath.Join(".", ".stacked")
		err := os.MkdirAll(newpath, os.ModePerm)
		if err != nil {
			log.Fatalf("Unable to create .stacked dir")
		}

		if _, err := os.Stat(".stacked/main.db"); errors.Is(err, os.ErrNotExist) {
			// Create DB file
			log.Println("Creating main.db...")
			file, err := os.Create(filepath.Join(".stacked", "main.db"))
			if err != nil {
				log.Fatal(err.Error())
			}
			file.Close()
			log.Println("main.db created")
		}

		db, err := sqlx.Open("sqlite3", filepath.Join(".stacked", "main.db"))
		if err != nil {
			log.Fatalf("Unable to connect to database: %v\n", err)
		}

		schema := `
			CREATE TABLE IF NOT EXISTS diffs (
				id TEXT PRIMARY KEY,
				branch TEXT,
				pr_number TEXT NULL,
				stacked_on TEXT NULL
			);
			CREATE UNIQUE INDEX IF NOT EXISTS idx_diffs_id ON diffs (id);
		`

		// execute a query on the server
		db.MustExec(schema)

		// Get base branch name
		gitCmd := exec.Command("gh", "repo", "view", "--json=defaultBranchRef", "--jq=.defaultBranchRef.name")
		defaultBranch, err := runCommand("Get HEAD branch name", gitCmd, true)
		if err != nil {
			log.Fatalf(err.Error())
		}
		defaultBranch = strings.TrimSuffix(defaultBranch, "\n")

		config := &Config{
			DefaultBranch: defaultBranch,
		}

		disablePushCmd := exec.Command("git", "config", fmt.Sprintf("branch.%s.pushRemote", defaultBranch), "no_push")
		_, err = runCommand("Disable push to master", disablePushCmd, true)
		if err != nil {
			log.Fatalf(err.Error())
		}

		d, err := yaml.Marshal(&config)
		if err != nil {
			log.Fatalf("error: %v", err)
		}
		err = os.WriteFile(filepath.Join(".stacked", "config.yaml"), d, 0644)
		if err != nil {
			log.Fatalf("error: %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
