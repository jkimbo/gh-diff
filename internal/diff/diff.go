package diff

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os/exec"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/jkimbo/stacked/internal/client"
	"github.com/jkimbo/stacked/internal/db"
	"github.com/jkimbo/stacked/utils"
)

func diffIDFromCommit(commit string) (string, error) {
	// Find diff trailer
	trailers, err := utils.RunCommand(
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

// Diff .
type Diff struct {
	ID         string
	Commit     string
	DBInstance *db.Diff
	client     *client.StackedClient
}

// Sync .
func (diff *Diff) Sync(ctx context.Context) error {
	commit, err := diff.GetCommit()
	if err != nil {
		return err
	}

	// Store current branch so that we can switch back to it later
	currentBranch := utils.MustRunCommand(
		"Get current branch",
		exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD"),
		true,
		false,
	)

	var syncError error
	if diff.IsSaved() == false {
		fmt.Println("commit hasn't been synced yet")
		syncError = diff.syncNew(ctx, commit)
	} else {
		fmt.Printf("diff already saved\n")
		syncError = diff.syncSaved(ctx, commit)
	}

	// Always switch back to "currentBranch"
	utils.MustRunCommand(
		"Switch back to current branch",
		exec.Command(
			"git", "switch", currentBranch,
		),
		true,
		false,
	)

	if syncError != nil {
		return err
	}

	if diff.HasPR() == false {
		err = diff.CreatePR(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

func (diff *Diff) syncNew(ctx context.Context, commit string) error {
	baseRef := fmt.Sprintf("origin/%s", diff.client.DefaultBranch())

	branchName, err := diff.generateBranchName()
	if err != nil {
		return err
	}

	var stackedOn string

	// Check parent commit to see if it's also a diff
	parentCommit, err := utils.RunCommand(
		"Get parent commit",
		exec.Command(
			"git", "rev-parse", fmt.Sprintf("%s^", commit),
		),
		true,
		false,
	)
	if err != nil {
		return err
	}

	parentDiffID, err := diffIDFromCommit(parentCommit)
	if err != nil {
		return err
	}

	if parentDiffID != "" {
		fmt.Println("parent commit is a diff")
		parentDiff, err := NewDiffFromID(ctx, diff.client, parentDiffID)
		if err != nil {
			return err
		}

		// If the parent diff hasn't been saved then assume the baseRef is the
		// default branch
		if parentDiff.IsSaved() == true {
			if merged, err := parentDiff.IsMerged(); err != nil {
				return err
			} else if merged {
				fmt.Println("parent commit has already been merged")
			} else {
				prompt := &survey.Select{
					Message: fmt.Sprintf("Base?"),
					Options: []string{
						fmt.Sprintf("%s (%s)", parentDiff.GetSubject(), parentDiff.ID),
						fmt.Sprintf("origin/%s", diff.client.DefaultBranch()),
					},
				}
				err := survey.AskOne(prompt, &baseRef)
				if err != nil {
					if err == terminal.InterruptErr {
						log.Fatal("interrupted")
					}
					log.Fatalf("err: %s", err)
				}
				if baseRef == parentDiff.DBInstance.Branch {
					stackedOn = parentDiff.ID
				}
			}
		}
	}

	fmt.Printf("syncing %s to branch %s (base: %s)\n", commit, branchName, baseRef)

	err = diff.SyncCommitToBranch(ctx, commit, branchName, baseRef)
	if err != nil {
		return err
	}

	// Save diff
	err = diff.client.SQLDB.CreateDiff(ctx, &db.Diff{
		ID:        diff.ID,
		Branch:    branchName,
		StackedOn: stackedOn,
	})
	if err != nil {
		return err
	}

	dbDiff, err := diff.client.SQLDB.GetDiff(ctx, diff.ID)
	if err != nil {
		return err
	}
	diff.DBInstance = dbDiff

	return nil
}

func (diff *Diff) syncSaved(ctx context.Context, commit string) error {
	baseRef := fmt.Sprintf("origin/%s", diff.client.DefaultBranch())

	stackedOnDiff, err := diff.StackedOn(ctx)
	if stackedOnDiff != nil {
		if merged, err := stackedOnDiff.IsMerged(); err != nil {
			return err
		} else if merged {
			fmt.Println("stacked diff has already been merged")
		} else {
			if stackedOnDiff.IsSaved() == false {
				return fmt.Errorf("stacked diff hasn't been synced")
			}

			fmt.Printf("stacked on %s\n", stackedOnDiff.DBInstance.ID)
			baseRef = stackedOnDiff.DBInstance.Branch
		}
	}

	err = diff.SyncCommitToBranch(ctx, commit, diff.DBInstance.Branch, baseRef)
	if err != nil {
		return err
	}

	return nil
}

func (diff *Diff) getStack(ctx context.Context) (*Stack, error) {
	st, err := NewStackFromDiff(ctx, diff)
	if err != nil {
		return nil, err
	}
	return st, nil
}

// SyncCommitToBranch .
func (diff *Diff) SyncCommitToBranch(ctx context.Context, commit, branchName, baseRef string) error {
	commitDate, err := utils.RunCommand(
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

	committerName, err := utils.RunCommand(
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

	committerEmail, err := utils.RunCommand(
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
	utils.RunCommand(
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

	_, err = utils.RunCommand(
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

	_, err = utils.RunCommand(
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

	cherryPickMsg, err := utils.RunCommand(
		"Cherry pick commit",
		cmd,
		true,
		false,
	)
	if err != nil {
		cherryPickErr := err

		_, err = utils.RunCommand(
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

		return fmt.Errorf("cherry-pick failed: %v\n%s", cherryPickErr, cherryPickMsg)
	}

	_, err = utils.RunCommand(
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

	return nil
}

// CreatePR .
func (diff *Diff) CreatePR(ctx context.Context) error {
	st, err := diff.getStack(ctx)
	if err != nil {
		return err
	}

	fmt.Printf("\nCreate a PR 🔽\n\thttps://github.com/frontedxyz/synthwave/compare/%s...%s\n\n", diff.client.DefaultBranch(), diff.GetBranch())

	table, err := st.buildTable()
	if err != nil {
		log.Fatalf("err: %v\n", err)
	}
	fmt.Printf("PR description:\n\n%s\n\n", table)

	return nil
}

// GetDependantDiffs .
func (diff *Diff) GetDependantDiffs(ctx context.Context) ([]*Diff, error) {
	st, err := diff.getStack(ctx)
	if err != nil {
		return nil, err
	}

	return st.DependantDiffs(ctx, diff)
}

func (diff *Diff) generateBranchName() (string, error) {
	var commit string
	commit, err := diff.GetCommit()
	if err != nil {
		return commit, err
	}

	branchName, err := utils.RunCommand(
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
		return commit, err
	}

	return branchName, nil
}

// GetCommit .
func (diff *Diff) GetCommit() (string, error) {
	if diff.Commit != "" {
		return diff.Commit, nil
	}
	// Find the commit of the current diff
	// Note: this can and will change as diffs get rebased regularly

	// Loop through all commits between HEAD and base branch
	commits, err := utils.RunCommand(
		"Get commits",
		exec.Command(
			"git",
			"rev-list",
			"--reverse",
			fmt.Sprintf("origin/%s...HEAD", diff.client.Config.DefaultBranch),
		),
		true,
		false,
	)
	if err != nil {
		return "", err
	}

	lines := strings.Split(commits, "\n")
	var commit string
	// TODO raise error if multiple diff ids found
	for _, line := range lines {
		diffID, err := diffIDFromCommit(line)
		if err != nil {
			// Ignore errors
			continue
		}
		if diffID == diff.ID {
			commit = line
			break
		}
	}

	if commit == "" {
		return commit, fmt.Errorf("can't find commit for diff %s", diff.ID)
	}

	diff.Commit = commit

	return commit, nil
}

// GetSubject .
func (diff *Diff) GetSubject() string {
	commit, err := diff.GetCommit()
	if err != nil {
		panic(err.Error())
	}

	subject := utils.MustRunCommand(
		"Get commit subject",
		exec.Command(
			"git",
			"show",
			"-s",
			"--format=%s",
			commit,
		),
		true,
		false,
	)
	return subject
}

// GetBranch .
func (diff *Diff) GetBranch() string {
	return diff.DBInstance.Branch
}

// IsMerged returns true if the diff has already been merged
func (diff *Diff) IsMerged() (bool, error) {
	commit, err := diff.GetCommit()
	if err != nil {
		return false, err
	}

	c := diff.client

	// check if diff has already been merged
	// https://git-scm.com/docs/git-cherry
	numCommits, err := utils.RunCommand(
		"Num commits yet to be applied",
		exec.Command(
			"bash",
			"-c",
			fmt.Sprintf("git cherry origin/%s %s | grep '+' | wc -l", c.DefaultBranch(), commit),
		),
		true,
		false,
	)
	if err != nil {
		return false, err
	}

	if numCommits == "0" {
		return true, nil
	}

	return false, nil
}

// IsSaved returns true if the diff has been persisted to the db
func (diff *Diff) IsSaved() bool {
	if diff.DBInstance == nil {
		return false
	}
	return true
}

// HasPR .
func (diff *Diff) HasPR() bool {
	return diff.DBInstance.PRNumber != ""
}

// StackedOn .
func (diff *Diff) StackedOn(ctx context.Context) (*Diff, error) {
	if diff.IsSaved() == false {
		return nil, fmt.Errorf("diff hasn't been synced: %s", diff.ID)
	}

	inst := diff.DBInstance

	if inst.StackedOn == "" {
		return nil, nil
	}

	stackedOnDiff, err := NewDiffFromID(ctx, diff.client, inst.StackedOn)
	if err != nil {
		return nil, err
	}
	if stackedOnDiff.IsSaved() == false {
		return nil, fmt.Errorf("diff hasn't been synced: %s", stackedOnDiff.ID)
	}
	return stackedOnDiff, nil
}

// ChildDiff .
func (diff *Diff) ChildDiff(ctx context.Context) (*Diff, error) {
	if diff.IsSaved() == false {
		return nil, fmt.Errorf("diff hasn't been synced: %s", diff.ID)
	}

	childDiff, err := diff.client.SQLDB.GetChildDiff(ctx, diff.ID)
	if err != nil {
		if err != sql.ErrNoRows {
			return nil, err
		} else {
			return nil, nil
		}
	}

	if childDiff != nil {
		child, err := NewDiffFromID(ctx, diff.client, childDiff.ID)
		if err != nil {
			return nil, err
		}
		return child, nil
	}
	return nil, nil
}

// NewDiffFromID .
func NewDiffFromID(ctx context.Context, c *client.StackedClient, diffID string) (*Diff, error) {
	var dbDiff *db.Diff
	dbDiff, err := c.SQLDB.GetDiff(ctx, diffID)
	if err != nil {
		if err != sql.ErrNoRows {
			return nil, err
		}
	}

	return &Diff{
		ID:         diffID,
		DBInstance: dbDiff,
		client:     c,
	}, nil
}

// NewDiffFromCommit .
func NewDiffFromCommit(ctx context.Context, c *client.StackedClient, commit string) (*Diff, error) {
	// Check that commit is valid
	_, err := utils.RunCommand(
		"Checking commit is valid",
		exec.Command("git", "cat-file", "-e", commit),
		false,
		false,
	)
	if err != nil {
		return nil, err
	}

	// Find diff trailer
	diffID, err := diffIDFromCommit(commit)
	if err != nil {
		return nil, err
	}

	if diffID == "" {
		return nil, fmt.Errorf("commit is missing a DiffID")
	}

	return NewDiffFromID(ctx, c, diffID)
}
