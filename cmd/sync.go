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

	"github.com/AlecAivazis/survey/v2"
	"github.com/jkimbo/stacked/db"
	"github.com/jkimbo/stacked/util"
	"github.com/spf13/cobra"
)

func syncDiff(commit, branchName, baseRef, currentBranch string) error {
	commitDate, err := util.RunCommand(
		"Get commit date",
		exec.Command(
			"git", "show", "-s", "--format=%ci", commit,
		),
		true,
		false,
	)
	if err != nil {
		return err
	}

	committerName, err := util.RunCommand(
		"Get committer name",
		exec.Command(
			"git", "show", "-s", "--format=%cn", commit,
		),
		true,
		false,
	)
	if err != nil {
		return err
	}

	committerEmail, err := util.RunCommand(
		"Get committer email",
		exec.Command(
			"git", "show", "-s", "--format=%ce", commit,
		),
		true,
		false,
	)
	if err != nil {
		return err
	}

	// Note: we don't care if this command fails
	util.RunCommand(
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
		false,
	)

	_, err = util.RunCommand(
		"Create new branch",
		exec.Command(
			"git", "branch", "--no-track", branchName, baseRef,
		),
		true,
		false,
	)
	if err != nil {
		return err
	}

	_, err = util.RunCommand(
		"Switch to branch",
		exec.Command(
			"git", "switch", branchName,
		),
		true,
		false,
	)
	if err != nil {
		return err
	}

	cmd := exec.Command(
		"git", "cherry-pick", commit,
	)
	cmd.Env = append(cmd.Env, fmt.Sprintf("GIT_COMMITTER_NAME=%s", committerName))
	cmd.Env = append(cmd.Env, fmt.Sprintf("GIT_COMMITTER_EMAIL=%s", committerEmail))
	cmd.Env = append(cmd.Env, fmt.Sprintf("GIT_COMMITTER_DATE=%s", commitDate))

	cherryPickMsg, err := util.RunCommand(
		"Cherry pick commit",
		cmd,
		true,
		false,
	)
	if err != nil {
		cherryPickErr := err

		_, err = util.RunCommand(
			"Abort cherry pick",
			exec.Command(
				"git", "cherry-pick", "--abort",
			),
			true,
			false,
		)
		if err != nil {
			return err
		}

		_, err = util.RunCommand(
			"Switch to branch",
			exec.Command(
				"git", "switch", currentBranch,
			),
			true,
			false,
		)
		if err != nil {
			return err
		}
		log.Printf("cherry-pick failed: %v\n", cherryPickMsg)
		return cherryPickErr
	}

	_, err = util.RunCommand(
		"Push branch",
		exec.Command(
			"git", "push", "origin", branchName, "--force",
		),
		false,
		false,
	)
	if err != nil {
		return err
	}

	_, err = util.RunCommand(
		"Switch back to current branch",
		exec.Command(
			"git", "switch", currentBranch,
		),
		true,
		false,
	)
	if err != nil {
		return err
	}

	return nil
}

func diffIDFromCommit(commit string) (string, error) {
	// Find diff trailer
	trailers, err := util.RunCommand(
		"Get commit trailers",
		exec.Command(
			"bash",
			"-c",
			fmt.Sprintf("git show -s --format=%%B %s | git interpret-trailers --parse", commit),
		),
		true,
		false,
	)
	if err != nil {
		return "", err
	}

	lines := strings.Split(trailers, "\n")
	var diffID string
	// TODO raise error if multiple diff ids found
	for _, line := range lines {
		kv := strings.Split(strings.TrimSpace(line), ":")
		if kv[0] == "DiffID" {
			diffID = strings.TrimSpace(kv[1])
			break
		}
	}

	return diffID, nil
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
		_, err := util.RunCommand(
			"Checking commit is valid",
			exec.Command("git", "cat-file", "-e", commit),
			false,
			false,
		)
		if err != nil {
			os.Exit(1)
		}

		fmt.Println("Syncing diff:", commit)

		// Find diff trailer
		diffID, err := diffIDFromCommit(commit)
		if err != nil {
			log.Fatalf("err: %s", err)
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
		currentBranch, err := util.RunCommand(
			"Get current branch",
			exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD"),
			true,
			false,
		)

		diff, err := sqlDB.GetDiff(ctx, diffID)
		if err != nil {
			if err != sql.ErrNoRows {
				log.Fatalf("error: %v", err)
			}
			fmt.Println("Commit hasn't been synced yet")

			// Determine branch name
			branchName, err := util.RunCommand(
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
				false,
			)
			if err != nil {
				log.Fatalf("error: %v", err)
			}

			var baseRef string
			// TODO check parent commit to see if it's also a diff
			parentCommit, err := util.RunCommand(
				"Get parent commit",
				exec.Command(
					"git", "rev-parse", fmt.Sprintf("%s^", commit),
				),
				true,
				false,
			)
			if err != nil {
				log.Fatalf("error: %v", err)
			}

			parentDiffID, err := diffIDFromCommit(parentCommit)
			if err != nil {
				log.Fatalf("error: %v", err)
			}

			var stackedOn string
			if parentDiffID != "" {
				parentDiff, err := sqlDB.GetDiff(ctx, parentDiffID)
				if err != nil {
					if err != sql.ErrNoRows {
						log.Fatalf("error: %v", err)
					}
					baseRef = fmt.Sprintf("origin/%s", config.DefaultBranch)
				} else {
					prompt := &survey.Select{
						Message: fmt.Sprintf("Base?"),
						Options: []string{
							parentDiff.Branch,
							fmt.Sprintf("origin/%s", config.DefaultBranch),
						},
					}
					survey.AskOne(prompt, &baseRef)
					if baseRef == parentDiff.Branch {
						stackedOn = parentDiff.ID
					}
				}
			}

			log.Printf("Syncing %s to branch %s\n", commit, branchName)

			err = syncDiff(commit, branchName, baseRef, currentBranch)
			if err != nil {
				log.Fatalf("error: %v", err)
			}

			// save diff
			err = sqlDB.CreateDiff(ctx, &db.Diff{
				ID:        diffID,
				Branch:    branchName,
				StackedOn: stackedOn,
			})
			if err != nil {
				log.Fatalf("error: %v", err)
			}

			return
		}

		log.Print("Diff already exists")

		var baseRef string
		if diff.StackedOn != "" {
			parentDiff, err := sqlDB.GetDiff(ctx, diff.StackedOn)
			if err != nil {
				log.Fatalf("error: %v", err)
			}

			// check if diff has already been merged
			// https://git-scm.com/docs/git-cherry
			numCommits, err := util.RunCommand(
				"Num commits yet to be applied",
				exec.Command(
					"bash",
					"-c",
					fmt.Sprintf("git cherry origin/%s %s | grep '+' | wc -l", config.DefaultBranch, parentDiff.Branch),
				),
				true,
				false,
			)
			if err != nil {
				log.Fatalf("error: %v", err)
			}

			if numCommits == "0" {
				baseRef = fmt.Sprintf("origin/%s", config.DefaultBranch)
			} else {
				baseRef = parentDiff.Branch
			}
		} else {
			baseRef = fmt.Sprintf("origin/%s", config.DefaultBranch)
		}

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
