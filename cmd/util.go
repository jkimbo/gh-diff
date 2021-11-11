package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func printCmd(description string, cmd *exec.Cmd) {
	for _, v := range cmd.Env {
		fmt.Printf("%s \\\n", v)
	}
	fmt.Printf("+ %s", cmd)
	fmt.Printf(" # %s", description)
	fmt.Println("")
}

func runCommand(description string, cmd *exec.Cmd, capture bool) (string, error) {
	// TODO only print when verbose flag is set
	printCmd(description, cmd)
	var out []byte
	var err error
	if capture {
		out, err = cmd.CombinedOutput()

		output := string(out)
		output = strings.TrimSuffix(output, "\n")
		fmt.Println("#", output)

		if err != nil {
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
