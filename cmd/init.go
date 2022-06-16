package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jkimbo/gh-diff/internal/config"
	"github.com/jkimbo/gh-diff/internal/db"
	"github.com/jkimbo/gh-diff/utils"
	_ "github.com/mattn/go-sqlite3" // so that sqlx works with sqlite
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialise stacked",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Initialise stacked")

		ctx := context.Background()

		// find root path
		path, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
		if err != nil {
			log.Fatalf("cannot find git folder")
		}
		rootPath := strings.TrimSpace(string(path))

		newpath := filepath.Join(rootPath, ".stacked")
		err = os.MkdirAll(newpath, os.ModePerm)
		if err != nil {
			log.Fatalf("Unable to create .stacked dir")
		}

		if _, err := os.Stat(filepath.Join(rootPath, ".stacked", "main.db")); errors.Is(err, os.ErrNotExist) {
			// Create DB file
			fmt.Println("Creating main.db...")
			file, err := os.Create(filepath.Join(".stacked", "main.db"))
			if err != nil {
				log.Fatal(err.Error())
			}
			file.Close()
			fmt.Println("main.db created")
		}

		db, err := db.NewDB(ctx, filepath.Join(rootPath, ".stacked", "main.db"))
		if err != nil {
			log.Fatalf("Unable to connect to database: %v\n", err)
		}

		err = db.Init(ctx)
		if err != nil {
			log.Fatalf("error setting up db: %v", err)
		}

		// Get base branch name
		gitCmd := exec.Command("gh", "repo", "view", "--json=defaultBranchRef", "--jq=.defaultBranchRef.name")
		defaultBranch, err := utils.RunCommand("Get HEAD branch name", gitCmd, true, false)
		if err != nil {
			log.Fatalf(err.Error())
		}

		config := &config.Config{
			DefaultBranch: defaultBranch,
		}

		// disablePushCmd := exec.Command("git", "config", fmt.Sprintf("branch.%s.pushRemote", defaultBranch), "no_push")
		// _, err = runCommand("Disable push to master", disablePushCmd, false)
		// if err != nil {
		// 	log.Fatalf(err.Error())
		// }

		// Set pull.rebase to true
		_, err = utils.RunCommand(
			"Set config pull.rebase to true",
			exec.Command("git", "config", "pull.rebase", "true"),
			false,
			false,
		)
		if err != nil {
			log.Fatalf(err.Error())
		}

		d, err := yaml.Marshal(&config)
		if err != nil {
			log.Fatalf("error: %v", err)
		}
		err = os.WriteFile(filepath.Join(rootPath, ".stacked", "config.yaml"), d, 0644)
		if err != nil {
			log.Fatalf("error: %v", err)
		}

		// Setup git commit hook
		commitMsgHookPath := filepath.Join(rootPath, ".git", "hooks", "commit-msg")
		if _, err := os.Stat(commitMsgHookPath); err == nil {
			log.Fatalf("commit-msg hook already exists")
		}

		resp, err := http.Get("https://raw.githubusercontent.com/jkimbo/stacked/main/hooks/commit-msg")
		if err != nil {
			log.Fatalf("err downloading hook: %s", err)
		}
		defer resp.Body.Close()

		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Fatal(err)
		}

		err = ioutil.WriteFile(commitMsgHookPath, bodyBytes, 0755)
		if err != nil {
			log.Fatalf("err writing hook: %s", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
