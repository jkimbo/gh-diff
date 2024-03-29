package diff

import (
	"fmt"
	"os/exec"
	"strings"
)

type gitcmd struct {
}

func (c *gitcmd) getPatch(ref string, base string) string {
	cmd := exec.Command(
		"git", "diff", "--no-ext-diff", "--unified=0", base, ref,
	)

	rawCommitContents := mustCommand(
		cmd,
		true,
		false,
	)

	var commitContents strings.Builder
	// filter out index lines
	lines := strings.Split(rawCommitContents, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "index") {
			continue
		}
		commitContents.WriteString(line)
		commitContents.WriteString("\n")
	}

	return commitContents.String()
}

func (c *gitcmd) getPatchID(ref string) string {
	rawCmd := fmt.Sprintf("git show %s | git patch-id --stable", ref)

	cmd := exec.Command(
		"bash", "-c", rawCmd,
	)

	output := mustCommand(
		cmd,
		true,
		false,
	)

	parts := strings.Split(output, " ")
	return parts[0]
}

func (c *gitcmd) getMergeBase(commitA string, commitB string) string {
	cmd := exec.Command(
		"git", "merge-base", commitA, commitB,
	)

	mergeBase := mustCommand(
		cmd,
		true,
		false,
	)

	return mergeBase
}
