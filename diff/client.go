package diff

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
)

var client *diffclient

type diffclient struct {
	db     *SQLDB
	config *config
}

func (c *diffclient) Setup(ctx context.Context) error {
	sqlDB, err := NewDB(ctx, filepath.Join(".diff", "main.db"))
	if err != nil {
		return fmt.Errorf("Unable to connect to database: %v\n", err)
	}
	c.db = sqlDB

	config, err := loadConfig()
	if err != nil {
		return err
	}
	c.config = config
	return nil
}

func (c *diffclient) SyncDiff(ctx context.Context, commit string) error {
	d, err := newDiffFromCommit(ctx, commit)
	check(err)

	fmt.Printf("syncing diff: %s (%s)\n", d.GetSubject(), d.ID)

	err = d.Sync(ctx)
	check(err)

	if d.HasPR() == false {
		err = d.CreatePR(ctx)
		check(err)
	}

	dependantDiffs, err := d.GetDependantDiffs(ctx)
	check(err)

	if len(dependantDiffs) > 0 {
		fmt.Printf("%d dependant diffs to sync\n", len(dependantDiffs))

		for _, dependantDiff := range dependantDiffs {
			fmt.Printf("syncing dependant diff: %s (%s)\n", dependantDiff.GetSubject(), dependantDiff.ID)
			err = dependantDiff.Sync(ctx)
			check(err)
		}
	}

	// TODO update all PR descriptions and titles in the stack

	return nil
}

func (c *diffclient) Init(ctx context.Context) error {
	// find root path
	path, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		log.Fatalf("cannot find git folder")
	}
	rootPath := strings.TrimSpace(string(path))

	newpath := filepath.Join(rootPath, ".diff")
	err = os.MkdirAll(newpath, os.ModePerm)
	if err != nil {
		log.Fatalf("Unable to create .diff dir")
	}

	if _, err := os.Stat(filepath.Join(rootPath, ".diff", "main.db")); errors.Is(err, os.ErrNotExist) {
		// Create DB file
		fmt.Println("Creating main.db...")
		file, err := os.Create(filepath.Join(".diff", "main.db"))
		if err != nil {
			log.Fatal(err.Error())
		}
		file.Close()
		fmt.Println("main.db created")
	}

	db, err := NewDB(ctx, filepath.Join(rootPath, ".diff", "main.db"))
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}

	err = db.Init(ctx)
	if err != nil {
		log.Fatalf("error setting up db: %v", err)
	}

	initConfig(rootPath)

	// disablePushCmd := exec.Command("git", "config", fmt.Sprintf("branch.%s.pushRemote", defaultBranch), "no_push")
	// _, err = runCommand("Disable push to master", disablePushCmd, false)
	// if err != nil {
	// 	log.Fatalf(err.Error())
	// }

	// Set pull.rebase to true
	mustCommand(
		exec.Command("git", "config", "pull.rebase", "true"),
		false,
		false,
	)

	// Setup git commit hook
	commitMsgHookPath := filepath.Join(rootPath, ".git", "hooks", "commit-msg")
	if _, err := os.Stat(commitMsgHookPath); err != nil {
		resp, err := http.Get("https://raw.githubusercontent.com/jkimbo/gh-diff/main/hooks/commit-msg")
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
	} else {
		fmt.Println("commit-msg hook already exists. skipping")
	}

	return nil
}

func (c *diffclient) LandDiff(ctx context.Context, commit string) error {
	diff, err := newDiffFromCommit(ctx, commit)
	check(err)

	isMerged := diff.IsMerged()
	if isMerged == true {
		log.Fatalf("commit already landed")
	}

	// Make sure that diff is not dependant on another diff that hasn't landed
	// yet
	stackedOnDiff, err := diff.StackedOn(ctx)
	check(err)
	if stackedOnDiff != nil {
		isMerged := stackedOnDiff.IsMerged()
		if !isMerged {
			fmt.Printf("Diff is stacked on %s that hasn't landed yet\n", diff.DBInstance.StackedOn)
			os.Exit(1)
		}
	}

	if diff.DBInstance.PRNumber == "" {
		fmt.Printf("Diff %s doesn't have a PR number\n", diff.ID)
		os.Exit(1)
		return nil
	}

	fmt.Printf("Landing commit: %s", commit)

	// Merge PR
	_, _, err = ghCommand(
		[]string{
			"pr", "merge", diff.DBInstance.PRNumber, "--squash",
		},
	)

	mustCommand(
		exec.Command(
			"git", "pull", "origin", c.config.DefaultBranch, "--rebase",
		),
		true,
		false,
	)

	// TODO
	// childDiff, err := sqlDB.GetChildDiff(ctx, diff.ID)
	// if err != nil {
	// 	if err != sql.ErrNoRows {
	// 		log.Fatalf("error: %v", err)
	// 	}
	// } else {
	// 	fmt.Println("Syncing child diffs")
	// 	childDiffID := childDiff.ID

	// 	// TODO sync all stacked on diffs
	// 	for childDiffID != "" {
	// 		// TODO Update PR base branch
	// 		// TODO Sync diff
	// 	}
	// 	fmt.Println("Syncing done")
	// }
	return nil
}

func NewClient() *diffclient {
	client = &diffclient{}
	return client
}
