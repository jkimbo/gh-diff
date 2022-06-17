package diff

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
)

func diffIDFromCommit(commit string) string {
	// Find diff trailer
	trailers := mustCommand(
		exec.Command(
			"bash",
			"-c",
			fmt.Sprintf("git show -s --format=%%B %s | git interpret-trailers --parse", commit),
		),
		true,
		false,
	)

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

	return diffID
}

// diff .
type Diff struct {
	ID         string
	Commit     string
	DBInstance *dbdiff
}

// Sync .
func (diff *Diff) Sync(ctx context.Context) error {
	var err error
	commit := diff.GetCommit()
	if commit == "" {
		return fmt.Errorf("can't find commit for diff %s", diff.ID)
	}

	// Store current branch so that we can switch back to it later
	currentBranch := mustCommand(
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
	mustCommand(
		exec.Command(
			"git", "switch", currentBranch,
		),
		true,
		false,
	)

	if syncError != nil {
		return err
	}

	return nil
}

func (diff *Diff) syncNew(ctx context.Context, commit string) error {
	baseRef := fmt.Sprintf("origin/%s", client.config.DefaultBranch)

	branchName, err := diff.generateBranchName()
	if err != nil {
		return err
	}

	var stackedOn string

	// Check parent commit to see if it's also a diff
	parentCommit, err := runCommand(
		exec.Command(
			"git", "rev-parse", fmt.Sprintf("%s^", commit),
		),
		true,
		false,
	)
	if err != nil {
		return err
	}

	parentDiffID := diffIDFromCommit(parentCommit)

	if parentDiffID != "" {
		fmt.Println("parent commit is a diff")
		parentDiff, err := newDiffFromID(ctx, parentDiffID)
		if err != nil {
			return err
		}

		// If the parent diff hasn't been saved then assume the baseRef is the
		// default branch
		if parentDiff.IsSaved() == true {
			merged := parentDiff.IsMerged()
			if merged {
				fmt.Println("parent commit has already been merged")
			} else {
				stackChanges := false
				prompt := &survey.Confirm{
					Message: fmt.Sprintf(
						"Stack your changes on \"[%s] %s\"?",
						parentDiff.ID, parentDiff.GetSubject(),
					),
				}
				err := survey.AskOne(prompt, &stackChanges)
				if err != nil {
					if err == terminal.InterruptErr {
						os.Exit(1)
					}
					log.Fatalf("err: %s", err)
				}
				if stackChanges == true {
					baseRef = parentDiff.DBInstance.Branch
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
	err = client.db.CreateDiff(ctx, &dbdiff{
		ID:        diff.ID,
		Branch:    branchName,
		StackedOn: stackedOn,
	})
	if err != nil {
		return err
	}

	dbDiff, err := client.db.GetDiff(ctx, diff.ID)
	if err != nil {
		return err
	}
	diff.DBInstance = dbDiff

	return nil
}

func (diff *Diff) syncSaved(ctx context.Context, commit string) error {
	baseRef := fmt.Sprintf("origin/%s", client.config.DefaultBranch)

	stackedOnDiff, err := diff.StackedOn(ctx)
	if stackedOnDiff != nil {
		if stackedOnDiff.IsSaved() == false {
			return fmt.Errorf("stacked diff hasn't been synced")
		}

		baseRef = stackedOnDiff.DBInstance.Branch
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
	var err error
	commitDate := mustCommand(
		exec.Command(
			"git", "show", "-s", "--format=%ci", commit,
		),
		true,
		false,
	)

	committerName := mustCommand(
		exec.Command(
			"git", "show", "-s", "--format=%cn", commit,
		),
		true,
		false,
	)

	committerEmail := mustCommand(
		exec.Command(
			"git", "show", "-s", "--format=%ce", commit,
		),
		true,
		false,
	)

	// Note: we don't care if this command fails
	runCommand(
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

	mustCommand(
		exec.Command(
			"git", "branch", "--no-track", branchName, baseRef,
		),
		true,
		false,
	)

	mustCommand(
		exec.Command(
			"git", "switch", branchName,
		),
		true,
		false,
	)

	cmd := exec.Command(
		"git", "cherry-pick", commit,
	)
	cmd.Env = append(cmd.Env, fmt.Sprintf("GIT_COMMITTER_NAME=%s", committerName))
	cmd.Env = append(cmd.Env, fmt.Sprintf("GIT_COMMITTER_EMAIL=%s", committerEmail))
	cmd.Env = append(cmd.Env, fmt.Sprintf("GIT_COMMITTER_DATE=%s", commitDate))

	cherryPickMsg, err := runCommand(
		cmd,
		true,
		false,
	)
	if err != nil {
		cherryPickErr := err

		_, err = runCommand(
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

	mustCommand(
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

	baseRef := client.config.DefaultBranch
	stackedOn, err := diff.StackedOn(ctx)
	if err != nil {
		return err
	}

	if stackedOn != nil {
		baseRef = stackedOn.GetBranch()
	}

	fmt.Printf("\nCreate a PR 🔽\n\thttps://github.com/frontedxyz/synthwave/compare/%s...%s\n\n", baseRef, diff.GetBranch())

	fmt.Printf("----\n")

	title := diff.GetSubject()

	if st.Size() > 1 {
		index := st.GetIndex(diff)
		title += fmt.Sprintf(" (%d/%d)", index+1, st.Size())
	}
	fmt.Printf("Title: %s\n", title)

	var body strings.Builder
	body.WriteString("Body:\n")
	body.WriteString(fmt.Sprintf("%s\n", diff.GetBody()))

	table, err := st.buildTable()
	if err != nil {
		log.Fatalf("err: %v\n", err)
	}
	if table != "" {
		body.WriteString(fmt.Sprintf("%s", table))
	}

	fmt.Println(body.String())

	fmt.Printf("----\n")

	return nil
}

// UpdatePRDescription .
func (diff *Diff) UpdatePRDescription(ctx context.Context) error {
	st, err := diff.getStack(ctx)
	if err != nil {
		return err
	}

	fmt.Printf("----\n")

	title := diff.GetSubject()

	if st.Size() > 1 {
		index := st.GetIndex(diff)
		title += fmt.Sprintf(" (%d/%d)", index+1, st.Size())
	}
	fmt.Printf("Title: %s\n", title)

	var body strings.Builder
	body.WriteString("Body:\n")
	body.WriteString(fmt.Sprintf("%s\n", diff.GetBody()))

	table, err := st.buildTable()
	if err != nil {
		log.Fatalf("err: %v\n", err)
	}
	if table != "" {
		body.WriteString(fmt.Sprintf("%s", table))
	}

	fmt.Println(body.String())

	fmt.Printf("----\n")

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
	commit = diff.GetCommit()
	if commit == "" {
		panic(fmt.Errorf("can't find commit for diff %s", diff.ID))
	}

	branchName, err := runCommand(
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
func (diff *Diff) GetCommit() string {
	if diff.Commit != "" {
		return diff.Commit
	}
	// Find the commit of the current diff
	// Note: this can and will change as diffs get rebased regularly

	// Loop through all commits between HEAD and base branch
	commits := mustCommand(
		exec.Command(
			"git",
			"rev-list",
			"--reverse",
			fmt.Sprintf("origin/%s...HEAD", client.config.DefaultBranch),
		),
		true,
		false,
	)

	lines := strings.Split(commits, "\n")
	var commit string
	// TODO raise error if multiple diff ids found
	for _, line := range lines {
		diffID := diffIDFromCommit(line)
		if diffID == diff.ID {
			commit = line
			break
		}
	}

	if commit == "" {
		return commit
	}

	diff.Commit = commit

	return commit
}

// GetSubject .
func (diff *Diff) GetSubject() string {
	commit := diff.GetCommit()
	if commit == "" {
		panic(fmt.Errorf("can't find commit for diff %s", diff.ID))
	}

	subject := mustCommand(
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

// GetBody .
func (diff *Diff) GetBody() string {
	commit := diff.GetCommit()
	if commit == "" {
		panic(fmt.Errorf("can't find commit for diff %s", diff.ID))
	}

	subject := mustCommand(
		exec.Command(
			"git",
			"show",
			"-s",
			"--format=%b",
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
func (diff *Diff) IsMerged() bool {
	commit := diff.GetCommit()
	if commit == "" {
		return true
	}

	// check if diff has already been merged
	// https://git-scm.com/docs/git-cherry
	numCommits := mustCommand(
		exec.Command(
			"bash",
			"-c",
			fmt.Sprintf("git cherry origin/%s %s | grep '+' | wc -l", client.config.DefaultBranch, commit),
		),
		true,
		false,
	)

	if numCommits == "0" {
		return true
	}

	return false
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

	stackedOnDiff, err := newDiffFromID(ctx, inst.StackedOn)
	if err != nil {
		return nil, err
	}

	if stackedOnDiff.IsSaved() == false {
		return nil, fmt.Errorf("diff hasn't been synced: %s", stackedOnDiff.ID)
	}

	merged := stackedOnDiff.IsMerged()
	if merged == true {
		return nil, nil
	}
	return stackedOnDiff, nil
}

// ChildDiff .
func (diff *Diff) ChildDiff(ctx context.Context) (*Diff, error) {
	if diff.IsSaved() == false {
		return nil, fmt.Errorf("diff hasn't been synced: %s", diff.ID)
	}

	childDiff, err := client.db.GetChildDiff(ctx, diff.ID)
	if err != nil {
		if err != sql.ErrNoRows {
			return nil, err
		} else {
			return nil, nil
		}
	}

	if childDiff != nil {
		child, err := newDiffFromID(ctx, childDiff.ID)
		if err != nil {
			return nil, err
		}
		return child, nil
	}
	return nil, nil
}

// newDiffFromID .
func newDiffFromID(ctx context.Context, diffID string) (*Diff, error) {
	instance, err := client.db.GetDiff(ctx, diffID)
	if err != nil {
		if err != sql.ErrNoRows {
			return nil, err
		}
	}

	return &Diff{
		ID:         diffID,
		DBInstance: instance,
	}, nil
}

// newDiffFromCommit .
func newDiffFromCommit(ctx context.Context, commit string) (*Diff, error) {
	// Check that commit is valid
	mustCommand(
		exec.Command("git", "cat-file", "-e", commit),
		false,
		false,
	)

	// Find diff trailer
	diffID := diffIDFromCommit(commit)

	if diffID == "" {
		return nil, fmt.Errorf("commit is missing a DiffID")
	}

	return newDiffFromID(ctx, diffID)
}
