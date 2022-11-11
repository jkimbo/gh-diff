package diff

import (
	"context"
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
		if kv[0] == "Diff-Id" {
			diffID = strings.TrimSpace(kv[1])
			break
		}
		if kv[0] == "DiffID" {
			diffID = strings.TrimSpace(kv[1])
			break
		}
	}

	return diffID
}

// diff .
type diff struct {
	id           string
	commit       string
	branch       string
	prNumber     string
	parentDiffID string
	git          *gitcmd
}

// Sync .
func (d *diff) Sync(ctx context.Context) error {
	var err error
	commit := d.commit
	if commit == "" {
		return fmt.Errorf("can't find commit for diff %s", d.id)
	}

	// Store current branch so that we can switch back to it later
	currentBranch := mustCommand(
		exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD"),
		true,
		false,
	)

	var syncError error
	if d.isSaved() == false {
		fmt.Println("commit hasn't been synced yet")
		syncError = d.syncNew(ctx, commit)
	} else {
		fmt.Printf("diff already saved\n")
		syncError = d.syncSaved(ctx, commit)
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

func (d *diff) syncNew(ctx context.Context, commit string) error {
	baseRef := fmt.Sprintf("origin/%s", client.config.DefaultBranch)

	branchName, err := d.generateBranchName()
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
		parentDiff, err := newDiffFromCommit(ctx, parentCommit)
		if err != nil {
			return err
		}

		// If the parent diff hasn't been saved then assume the baseRef is the
		// default branch
		if parentDiff.isSaved() == true {
			stackChanges := false
			prompt := &survey.Confirm{
				Message: fmt.Sprintf(
					"Stack your changes on \"[%s] %s\"?",
					parentDiff.id, parentDiff.getSubject(),
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
				baseRef = parentDiff.branch
				stackedOn = parentDiff.id
			}
		}
	}

	fmt.Printf("syncing %s to branch %s (base: %s)\n", commit, branchName, baseRef)

	err = d.syncCommitToBranch(ctx, commit, branchName, baseRef)
	if err != nil {
		return err
	}

	// Save diff
	err = client.db.createDiff(ctx, &dbdiff{
		ID:        d.id,
		Branch:    branchName,
		StackedOn: stackedOn,
	})
	if err != nil {
		return err
	}

	d.branch = branchName
	d.parentDiffID = stackedOn

	return nil
}

func (d *diff) syncSaved(ctx context.Context, commit string) error {
	baseRef := fmt.Sprintf("origin/%s", client.config.DefaultBranch)

	stackedOnDiff, err := d.parentDiff(ctx)
	if stackedOnDiff != nil && stackedOnDiff.commit != "" {
		if stackedOnDiff.isSaved() == false {
			return fmt.Errorf("stacked diff hasn't been synced")
		}

		baseRef = stackedOnDiff.branch
	}

	err = d.syncCommitToBranch(ctx, commit, d.branch, baseRef)
	if err != nil {
		return err
	}

	return nil
}

func (d *diff) getStack(ctx context.Context) (*stack, error) {
	st, err := newStackFromDiff(ctx, d)
	if err != nil {
		return nil, err
	}
	return st, nil
}

func (d *diff) syncCommitToBranch(ctx context.Context, commit, branchName, baseRef string) error {
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
			"git", "push", "origin", branchName, "--force-with-lease",
		),
		false,
		false,
	)
	if err != nil {
		return err
	}

	return nil
}

func (d *diff) createPR(ctx context.Context) error {
	if d.branch == "" {
		return fmt.Errorf("diff doesn't have a branch name")
	}

	baseRef := client.config.DefaultBranch
	stackedOn, err := d.parentDiff(ctx)
	if err != nil {
		return err
	}

	if stackedOn != nil && stackedOn.commit != "" {
		baseRef = stackedOn.branch
	}

	title := d.getSubject()
	body := d.getBody()

	prNumber, err := createPR(
		baseRef,
		d.branch,
		title,
		body,
	)
	if err != nil {
		return err
	}

	d.prNumber = prNumber
	// update db
	err = client.db.updatePrNumber(ctx, d.id, d.prNumber)
	if err != nil {
		return err
	}

	return nil

	/*
		repoURL := mustCommand(
			exec.Command("gh", "repo", "view", "--json=url", "--jq=.url"),
			true,
			false,
		)
		fmt.Printf("\nCreate a PR 🔽\n\t%s/compare/%s...%s\n\n", repoURL, baseRef, d.branch)

		fmt.Printf("----\n")

		title := d.getSubject()

		if st.size() > 1 {
			index := st.getIndex(d)
			title += fmt.Sprintf(" (%d/%d)", index+1, st.size())
		}
		fmt.Printf("Title: %s\n", title)

		var body strings.Builder
		body.WriteString("Body:\n")
		body.WriteString(fmt.Sprintf("%s\n", d.getBody()))

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
	*/
}

func (d *diff) updatePRDescription(ctx context.Context) error {
	st, err := d.getStack(ctx)
	if err != nil {
		return err
	}

	fmt.Printf("----\n")

	title := d.getSubject()

	if st.size() > 1 {
		index := st.getIndex(d)
		title += fmt.Sprintf(" (%d/%d)", index+1, st.size())
	}
	fmt.Printf("Title: %s\n", title)

	var body strings.Builder
	body.WriteString("Body:\n")
	body.WriteString(fmt.Sprintf("%s\n", d.getBody()))

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

func (d *diff) getDependantDiffs(ctx context.Context) ([]*diff, error) {
	st, err := d.getStack(ctx)
	if err != nil {
		return nil, err
	}

	return st.dependantDiffs(ctx, d)
}

func (d *diff) generateBranchName() (string, error) {
	commit := d.commit
	if commit == "" {
		panic(fmt.Errorf("can't find commit for diff %s", d.id))
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

func (d *diff) getSubject() string {
	commit := d.commit
	if commit == "" {
		panic(fmt.Errorf("can't find commit for diff %s", d.id))
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

func (d *diff) getBody() string {
	commit := d.commit
	if commit == "" {
		panic(fmt.Errorf("can't find commit for diff %s", d.id))
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

func (d *diff) isSaved() bool {
	if d.branch == "" {
		return false
	}
	return true
}

func (d *diff) parentDiff(ctx context.Context) (*diff, error) {
	if d.isSaved() == false {
		return nil, fmt.Errorf("diff hasn't been synced: %s", d.id)
	}

	if d.parentDiffID == "" {
		return nil, nil
	}

	stackedOnDiff, err := newDiffFromID(ctx, d.parentDiffID)
	if err != nil {
		return nil, err
	}

	return stackedOnDiff, nil
}

func (d *diff) childDiff(ctx context.Context) (*diff, error) {
	if d.isSaved() == false {
		return nil, fmt.Errorf("diff hasn't been synced: %s", d.id)
	}

	childDiff, err := client.db.getChildDiff(ctx, d.id)
	if err != nil {
		return nil, err
	}

	if childDiff == nil {
		return nil, nil
	}

	child, err := newDiffFromID(ctx, childDiff.ID)
	if err != nil {
		return nil, err
	}
	return child, nil
}

func (d *diff) needsSyncing(ctx context.Context) (bool, error) {
	// check if the diff needs syncing by diffing the commit contents against the
	// contents on the branch
	branch := d.branch
	if branch == "" {
		return false, fmt.Errorf("diff doesn't have a branch")
	}

	// get contents of the diff
	patchID1 := d.git.getPatchID(d.commit)
	patchID2 := d.git.getPatchID(d.branch)

	if patchID1 != patchID2 {
		return true, nil
	}

	return false, nil
}

func newDiffFromID(ctx context.Context, diffID string) (*diff, error) {
	// Find the commit of a diff
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
	for _, line := range lines {
		id := diffIDFromCommit(line)
		if id == diffID {
			commit = line
			break
		}
	}

	// Note: commit might be an empty string if the was merged or removed

	instance, err := client.db.getDiff(ctx, diffID)
	if err != nil {
		return nil, err
	}

	// the diff hasn't been saved to the db yet
	if instance == nil {
		return &diff{
			id:     diffID,
			commit: commit,
		}, nil
	}

	return &diff{
		id:           diffID,
		commit:       commit,
		branch:       instance.Branch,
		prNumber:     instance.PRNumber,
		parentDiffID: instance.StackedOn, // TODO: fix this naming inconsistency
	}, nil
}

func newDiffFromCommit(ctx context.Context, commit string) (*diff, error) {
	// Check that commit is valid
	mustCommand(
		exec.Command("git", "cat-file", "-e", commit),
		false,
		false,
	)

	// Find diff trailer
	diffID := diffIDFromCommit(commit)

	if diffID == "" {
		return nil, fmt.Errorf("commit is missing a Diff-Id")
	}

	instance, err := client.db.getDiff(ctx, diffID)
	if err != nil {
		return nil, err
	}

	// the diff hasn't been saved to the db yet
	if instance == nil {
		return &diff{
			id:     diffID,
			commit: commit,
		}, nil
	}

	return &diff{
		id:           diffID,
		commit:       commit,
		branch:       instance.Branch,
		prNumber:     instance.PRNumber,
		parentDiffID: instance.StackedOn, // TODO: fix this naming inconsistency
	}, nil
}
