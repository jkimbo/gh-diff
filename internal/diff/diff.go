package diff

import (
	"context"
	"database/sql"
	"fmt"
	"os/exec"
	"strings"

	"github.com/jkimbo/stacked/internal/db"
	"github.com/jkimbo/stacked/internal/client"
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
	DBInstance *db.Diff
	Client *client.StackedClient
}

// GetCommit .
func (diff *Diff) GetCommit(ctx context.Context) (string, error) {
	// Find the commit of the current diff
	// Note: this can and will change as diffs get rebased regularly

	// Loop through all commits between HEAD and base branch
	commits, err := utils.RunCommand(
		"Get commits",
		exec.Command(
			"git",
			"rev-list",
			"--reverse",
			fmt.Sprintf("origin/%s...HEAD", diff.Client.Config.DefaultBranch),
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

	return commit, nil
}

// IsMerged returns true if the diff has already been merged
func (diff *Diff) IsMerged(ctx context.Context) (bool, error) {
	commit, err := diff.GetCommit(ctx)
	if err != nil {
		return false, err
	}

	// check if diff has already been merged
	// https://git-scm.com/docs/git-cherry
	numCommits, err := utils.RunCommand(
		"Num commits yet to be applied",
		exec.Command(
			"bash",
			"-c",
			fmt.Sprintf("git cherry origin/%s %s | grep '+' | wc -l", diff.Client.Config.DefaultBranch, commit),
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

// StackedOn .
func (diff *Diff) StackedOn(ctx context.Context) (*Diff, error) {
	if diff.DBInstance.StackedOn == "" {
		return nil, nil
	}

	stackedOnDiff, err := LoadDiffFromID(ctx, diff.Client, diff.ID)
	if err != nil {
		return nil, err
	}
	return stackedOnDiff, nil
}

// LoadDiffFromCommit .
func LoadDiffFromCommit(ctx context.Context, c *client.StackedClient, commit string) (*Diff, error) {
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

	return LoadDiffFromID(ctx, c, diffID)
}

// LoadDiffFromID .
func LoadDiffFromID(ctx context.Context, c *client.StackedClient, diffID string) (*Diff, error) {
	dbDiff, err := c.SQLDB.GetDiff(ctx, diffID)
	if err != nil {
		if err != sql.ErrNoRows {
			return nil, err
		}
		return nil, fmt.Errorf("commit hasn't been synced yet")
	}

	return &Diff{
		ID:         diffID,
		DBInstance: dbDiff,
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

	var dbDiff *db.Diff
	dbDiff, err = c.SQLDB.GetDiff(ctx, diffID)
	if err != nil {
		if err != sql.ErrNoRows {
			return nil, err
		}
	}

	return &Diff{
		ID:         diffID,
		DBInstance: dbDiff,
		Client: c,
	}, nil
}
