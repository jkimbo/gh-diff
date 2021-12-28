package util

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/cli/go-gh"
)

var seededRand *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))

func RandomString(n int) string {
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

func RunCommand(description string, cmd *exec.Cmd, capture bool, verbose bool) (string, error) {
	// TODO only print when verbose flag is set
	if verbose == true {
		printCmd(description, cmd)
	}
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

func RunGHCommand(description string, args []string) (string, string, error) {
	stdOut, stdErr, err := gh.Exec(args...)
	if err != nil {
		fmt.Println(err)
		return stdOut.String(), stdErr.String(), nil
	}

	return stdOut.String(), stdErr.String(), nil
}
