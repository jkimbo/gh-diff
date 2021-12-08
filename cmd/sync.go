package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jkimbo/stacked/db"
	"github.com/spf13/cobra"
)

func syncDiff(commit, branchName, baseRef, currentBranch string) error {
	// Note: we don't care if this command fails
	runCommand(
		"Delete branch locally",
		exec.Command(
			"bash",
			"-c",
			fmt.Sprintf(
				"git branch -D %s",
				branchName,
			),
		),
		true,
	)

	_, err := runCommand(
		"Create new branch",
		exec.Command(
			"git", "branch", "--no-track", branchName, baseRef,
		),
		true,
	)
	if err != nil {
		return err
	}

	_, err = runCommand(
		"Switch to branch",
		exec.Command(
			"git", "switch", branchName,
		),
		true,
	)
	if err != nil {
		return err
	}

	cherryPickMsg, err := runCommand(
		"Cherry pick commit",
		exec.Command(
			"git", "cherry-pick", commit,
		),
		true,
	)
	if err != nil {
		cherryPickErr := err
		_, err = runCommand(
			"Abort cherry pick",
			exec.Command(
				"git", "cherry-pick", "--abort",
			),
			true,
		)
		if err != nil {
			return err
		}

		_, err = runCommand(
			"Switch to branch",
			exec.Command(
				"git", "switch", currentBranch,
			),
			true,
		)
		if err != nil {
			return err
		}
		log.Printf("cherry-pick failed: %v\n", cherryPickMsg)
		return cherryPickErr
	}

	_, err = runCommand(
		"Push branch",
		exec.Command(
			"git", "push", "origin", branchName, "--force",
		),
		false,
	)
	if err != nil {
		return err
	}

	_, err = runCommand(
		"Switch back to current branch",
		exec.Command(
			"git", "switch", currentBranch,
		),
		true,
	)
	if err != nil {
		return err
		log.Fatalf("error: %v", err)
	}

	return nil
}

var syncCmd = &cobra.Command{
	Use:   "sync [commit]",
	Short: "Sync a diff",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// TODO
		// * If diff has already been synced
		//   * Update diff and push it
		//   * Find all stacked diffs and update them as well
		// * Else
		//   * Check if parent commit is a diff
		//     * Base diff on top of parent diff
		//   * Else
		//     * Base of origin/{master,main}
		//   * Push diff and create PR
		//   * If stacked on another diff then add the list of stacked diffs to the PR description

		ctx := context.Background()
		commit := args[0]

		// Check that commit is valid
		_, err := runCommand(
			"Checking commit is valid",
			exec.Command("git", "cat-file", "-e", commit),
			false,
		)
		if err != nil {
			os.Exit(1)
		}

		fmt.Println("Syncing diff:", commit)

		// Find diff trailer
		trailers, err := runCommand(
			"Get commit trailers",
			exec.Command(
				"bash",
				"-c",
				fmt.Sprintf("git show -s --format=%%B %s | git interpret-trailers --parse", commit),
			),
			true,
		)
		if err != nil {
			log.Fatalf("error: %v", err)
		}

		lines := strings.Split(trailers, "\n")
		var diffID string
		for _, line := range lines {
			kv := strings.Split(strings.TrimSpace(line), ":")
			if kv[0] == "DiffID" {
				diffID = strings.TrimSpace(kv[1])
				break
			}
		}

		if diffID == "" {
			fmt.Println("Commit is missing a DiffID")
			os.Exit(1)
			return
		}

		sqlDB, err := db.NewDB(ctx, filepath.Join(".stacked", "main.db"))
		if err != nil {
			log.Fatalf("Unable to connect to database: %v\n", err)
		}

		config, err := loadConfig()
		if err != nil {
			log.Fatalf("err: %v\n", err)
		}

		// Store current branch so that we can switch back to it later
		currentBranch, err := runCommand(
			"Get current branch",
			exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD"),
			true,
		)

		baseRef := fmt.Sprintf("origin/%s", config.DefaultBranch)

		diff, err := sqlDB.GetDiff(ctx, diffID)
		if err != nil {
			if err != sql.ErrNoRows {
				log.Fatalf("error: %v", err)
			}
			fmt.Println("Commit hasn't been synced yet")

			// Determine branch name
			branchName, err := runCommand(
				"Generate branch name",
				exec.Command(
					"bash",
					"-c",
					fmt.Sprintf(
						"git show --no-patch --format=%%f %s | awk '{print tolower($0)}'",
						commit,
					),
				),
				true,
			)
			if err != nil {
				log.Fatalf("error: %v", err)
			}

			// TODO check parent commit to see if it's also a diff

			log.Printf("Syncing %s to branch %s\n", commit, branchName)

			err = syncDiff(commit, branchName, baseRef, currentBranch)
			if err != nil {
				log.Fatalf("error: %v", err)
			}

			// save diff
			err = sqlDB.CreateDiff(ctx, &db.Diff{
				ID:     diffID,
				Branch: branchName,
			})
			if err != nil {
				log.Fatalf("error: %v", err)
			}

			return
		}

		log.Print("Diff already exists")

		err = syncDiff(commit, diff.Branch, baseRef, currentBranch)
		if err != nil {
			log.Fatalf("error: %v", err)
		}

		// TODO sync any diffs that are stacked on top of this one

		// diffID = fmt.Sprintf("D%s", randomString(5))

		// execCmd = fmt.Sprintf(
		// 	"test $(git rev-parse HEAD) = '%s^') && git commit --amend --no-edit --trailer 'Diff-ID:%s' || true",
		// 	commit,
		// 	diffID,
		// )
		// _, err := runCommand(
		// 	"Tag commit with ID",
		// 	exec.Command(
		// 		"bash",
		// 		"-c",
		// 		fmt.Sprintf("git show -s --format=%%B %s | git interpret-trailers --parse", commit),
		// 	),
		// 	true,
		// )
		// if err != nil {
		// 	os.Exit(1)
		// }

		// Add diff
		// git rebase '5094f7d7af2a438b79a0420ccde237f161e9cbcd^' --exec 'test $(git rev-parse HEAD) = "5094f7d7af2a438b79a0420ccde237f161e9cbcd" && echo "hi" || true'
		// git commit --amend --no-edit --trailer "Test-Diff-ID:D1234"

	},
}

func init() {
	rootCmd.AddCommand(syncCmd)
}
