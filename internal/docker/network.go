package docker

import (
	"os/exec"
)

const SharedNetworkName = "sail_shared"

// NetworkExists checks if the shared Docker network exists.
func NetworkExists(name string) bool {
	return exec.Command("docker", "network", "inspect", name).Run() == nil
}

// CreateNetwork creates a Docker network.
func CreateNetwork(name string) error {
	return exec.Command("docker", "network", "create", name).Run()
}

// EnsureNetwork creates the network if it doesn't exist.
// Returns true if it was created.
func EnsureNetwork(name string) (bool, error) {
	if NetworkExists(name) {
		return false, nil
	}
	return true, CreateNetwork(name)
}
