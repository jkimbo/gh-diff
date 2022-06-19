package diff

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/cli/go-gh"
)

func check(err error) {
	if err != nil {
		if os.Getenv("GH_DIFF_DEBUG") == "1" {
			panic(err)
		}
		fmt.Printf("error: %s\n", err)
		os.Exit(1)
	}
}

var seededRand *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))

func randomString(n int) string {
	letters := "abcdefghijklmnopqrstuvwxyz1234567890"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[seededRand.Intn(len(letters))]
	}
	return string(b)
}

func printCmd(description string, cmd *exec.Cmd) {
	for _, v := range cmd.Env {
		fmt.Printf("%s \\\n", v)
	}
	fmt.Printf("+ %s", cmd)
	fmt.Printf(" # %s", description)
	fmt.Println("")
}

func runCommand(cmd *exec.Cmd, capture bool, verbose bool) (string, error) {
	var out []byte
	var err error
	if capture {
		out, err = cmd.CombinedOutput()

		output := string(out)
		output = strings.TrimSuffix(output, "\n")
		if verbose == true {
			fmt.Println("#", output)
		}

		if err != nil {
			fmt.Printf("# cmd: %v\n", cmd.Args)
			fmt.Println("#", output)
			fmt.Println("Error:", err)
			return "", err
		}

		return output, nil
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()

	if err != nil {
		fmt.Println("Error:", err)
		return "", err
	}

	return string(out), nil
}

func mustCommand(cmd *exec.Cmd, capture bool, verbose bool) string {
	output, err := runCommand(cmd, capture, verbose)
	check(err)

	return output
}

func ghCommand(args []string) (string, string, error) {
	stdOut, stdErr, err := gh.Exec(args...)
	if err != nil {
		fmt.Println(err)
		return stdOut.String(), stdErr.String(), nil
	}

	return stdOut.String(), stdErr.String(), nil
}
