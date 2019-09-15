package process

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

// RunEnv executes a given command in a subprocess and pipes all occurring outputs
// to stdout. It breaks on errors (from stderr)
func RunEnv(prefix string, envVars map[string]string, cmd []string) error {
	if len(cmd) == 0 {
		return fmt.Errorf("command length is zero")
	}

	c := exec.Command(cmd[0], cmd[1:]...)
	c.Env = os.Environ()
	for name, value := range envVars {
		c.Env = append(c.Env, fmt.Sprintf("%s=%s", name, value))
	}

	cReader, err := c.StdoutPipe()
	if err != nil {
		return fmt.Errorf(
			"error \"%s\" creating StdoutPipe for cmd: \"%s\"",
			err, strings.Join(cmd, " "))
	}

	scanner := bufio.NewScanner(cReader)
	go func() {
		for scanner.Scan() {
			LogInfo(prefix, scanner.Text())
		}
	}()
	err = c.Start()
	if err != nil {
		return fmt.Errorf("error \"%s\" starting cmd \"%s\"", err, strings.Join(cmd, " "))
	}
	err = c.Wait()
	if err != nil {
		return fmt.Errorf("error \"%s\" waiting for cmd \"%s\"", err, strings.Join(cmd, " "))
	}
	return nil
}

// RunEnvRes executes a given command in a subprocess and returns the result.
// It breaks on errors (from stderr)
func RunEnvRes(envVars map[string]string, cmd []string) (
	string, error,
) {
	if len(cmd) == 0 {
		return "", fmt.Errorf("command length is zero")
	}

	c := exec.Command(cmd[0], cmd[1:]...)
	c.Env = os.Environ()
	for name, value := range envVars {
		c.Env = append(c.Env, fmt.Sprintf("%s=%s", name, value))
	}

	outErrB, err := c.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("cmd \"%s\" failed - status code \"%s\", msg \"%s\"",
			cmd, err, string(outErrB))
	}
	return string(outErrB), nil
}

// LogInfo is used for fixing the style of log outputs
func LogInfo(prefix, entry string) {
	log.Printf("%s | %s", prefix, entry)
}
