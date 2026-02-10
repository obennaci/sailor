package git

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type Worktree struct {
	Path   string
	Branch string
	HEAD   string
	Bare   bool
}

// FindRoot returns the root directory of the main git repository.
func FindRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--git-common-dir").Output()
	if err != nil {
		return "", fmt.Errorf("not inside a Git repository")
	}
	gitCommon := strings.TrimSpace(string(out))
	root, err := filepath.Abs(filepath.Join(gitCommon, ".."))
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(root)
}

// ListWorktrees returns all git worktrees using porcelain format.
func ListWorktrees() ([]Worktree, error) {
	out, err := exec.Command("git", "worktree", "list", "--porcelain").Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	var worktrees []Worktree
	var current Worktree

	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "worktree "):
			if current.Path != "" {
				worktrees = append(worktrees, current)
			}
			current = Worktree{Path: strings.TrimPrefix(line, "worktree ")}
		case strings.HasPrefix(line, "HEAD "):
			current.HEAD = strings.TrimPrefix(line, "HEAD ")
		case strings.HasPrefix(line, "branch "):
			ref := strings.TrimPrefix(line, "branch ")
			current.Branch = strings.TrimPrefix(ref, "refs/heads/")
		case line == "bare":
			current.Bare = true
		}
	}
	if current.Path != "" {
		worktrees = append(worktrees, current)
	}

	return worktrees, nil
}

// Add creates a new git worktree.
func Add(root, targetDir, branch string) error {
	cmd := exec.Command("git", "-C", root, "worktree", "add", targetDir, branch)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

// Remove removes a git worktree.
func Remove(root, targetDir string) error {
	cmd := exec.Command("git", "-C", root, "worktree", "remove", targetDir, "--force")
	if err := cmd.Run(); err != nil {
		// Fallback: manual remove + prune
		exec.Command("rm", "-rf", targetDir).Run()
		return exec.Command("git", "-C", root, "worktree", "prune").Run()
	}
	return nil
}

// BranchExists checks if a branch exists.
func BranchExists(root, branch string) bool {
	err := exec.Command("git", "-C", root, "rev-parse", "--verify", branch).Run()
	return err == nil
}

// CreateBranch creates a new branch from HEAD.
func CreateBranch(root, branch string) error {
	return exec.Command("git", "-C", root, "branch", branch).Run()
}
