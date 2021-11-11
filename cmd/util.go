package cmd

import (
	"fmt"
	"os"
	"os/exec"
)

func printCmd(description string, cmd *exec.Cmd) {
	fmt.Println("Command:", description)
	for _, v := range cmd.Env {
		fmt.Printf("%s \\\n", v)
	}
	fmt.Printf("%s", cmd)
	fmt.Println("")
}

func runCommand(description string, cmd *exec.Cmd, capture bool) (string, error) {
	printCmd(description, cmd)
	var out []byte
	var err error
	if capture {
		out, err = cmd.CombinedOutput()
		fmt.Println("Output:", string(out))
	} else {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
	}

	if err != nil {
		fmt.Println("Error:", err)
		return "", err
	}

	return string(out), nil
}
