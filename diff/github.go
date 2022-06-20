package diff

import (
	"os/exec"
	"strconv"

	"github.com/shurcooL/githubv4"
)

func createPR(baseRef, branchName, title, body string) (prNumber string, err error) {
	repoID := mustCommand(
		exec.Command("gh", "repo", "view", "--json=id", "--jq=.id"),
		true,
		false,
	)

	var mutation struct {
		CreatePullRequest struct {
			PullRequest struct {
				ID     string
				Number int
			}
		} `graphql:"createPullRequest(input: $input)"`
	}

	variables := map[string]interface{}{
		"input": githubv4.CreatePullRequestInput{
			RepositoryID: githubv4.ID(repoID),
			BaseRefName:  githubv4.String(baseRef),
			HeadRefName:  githubv4.String(branchName),
			Title:        githubv4.String(title),
			Body:         githubv4.NewString(githubv4.String(body)),
		},
	}

	err = client.ghClient.Mutate("CreatePR", &mutation, variables)
	if err != nil {
		return "", err
	}

	return strconv.Itoa(mutation.CreatePullRequest.PullRequest.Number), err
}
