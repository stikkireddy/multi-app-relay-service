package app

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// findPID finds the PID of the process using the given port.
func findPID(port int) (int, error) {
	cmd := exec.Command("lsof", "-i", fmt.Sprintf(":%d", port))
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return 0, fmt.Errorf("could not execute lsof command: %v", err)
	}

	lines := strings.Split(out.String(), "\n")
	if len(lines) < 2 {
		return 0, fmt.Errorf("no process found using port %d", port)
	}

	fields := strings.Fields(lines[1])
	if len(fields) < 2 {
		return 0, fmt.Errorf("could not parse output for port %d", port)
	}

	pid, err := strconv.Atoi(fields[1])
	if err != nil {
		return 0, fmt.Errorf("invalid PID: %v", err)
	}

	return pid, nil
}

// killProcess kills the process with the given PID.
func killProcess(pid int) error {
	cmd := exec.Command("kill", "-9", strconv.Itoa(pid))
	return cmd.Run()
}

func KillPort(port int) error {
	pid, err := findPID(port)
	if err != nil {
		return err
	}
	return killProcess(pid)
}
