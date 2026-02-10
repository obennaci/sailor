package docker

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type ContainerInfo struct {
	Name    string `json:"Name"`
	Service string `json:"Service"`
	State   string `json:"State"`
	Status  string `json:"Status"`
	Ports   string `json:"Ports"`
}

// ComposePS runs docker compose ps in a directory and returns container info.
func ComposePS(dir string) ([]ContainerInfo, error) {
	cmd := exec.Command("docker", "compose", "ps", "--format", "json")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var containers []ContainerInfo
	// docker compose ps --format json outputs one JSON object per line
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		var c ContainerInfo
		if err := json.Unmarshal([]byte(line), &c); err != nil {
			continue
		}
		containers = append(containers, c)
	}
	return containers, nil
}

// ComposeUp starts services in a directory.
func ComposeUp(dir string, services ...string) error {
	args := []string{"compose", "up", "-d"}
	args = append(args, services...)
	cmd := exec.Command("docker", args...)
	cmd.Dir = dir
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

// ComposeDown stops services in a directory.
func ComposeDown(dir string) error {
	cmd := exec.Command("docker", "compose", "down")
	cmd.Dir = dir
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

// Exec runs a command inside a Docker container.
func Exec(container string, command ...string) (string, error) {
	args := append([]string{"exec", container}, command...)
	out, err := exec.Command("docker", args...).CombinedOutput()
	return string(out), err
}

// ExecStdin runs a command inside a container with stdin piped from another command.
func ExecStdin(container string, stdin string, command ...string) error {
	args := append([]string{"exec", "-i", container}, command...)
	cmd := exec.Command("docker", args...)
	cmd.Stdin = strings.NewReader(stdin)
	return cmd.Run()
}

// IsPortInUse checks if a docker container is listening on a host port.
func IsPortInUse(port int) bool {
	out, err := exec.Command("docker", "ps", "--format", "{{.Ports}}").Output()
	if err != nil {
		return false
	}
	search := fmt.Sprintf(":%d->", port)
	return strings.Contains(string(out), search)
}

// ComposeExec runs a command inside a compose service.
func ComposeExec(dir, service string, command ...string) error {
	args := append([]string{"compose", "exec", service}, command...)
	cmd := exec.Command("docker", args...)
	cmd.Dir = dir
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}
