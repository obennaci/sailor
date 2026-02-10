package deps

import (
	"os"
	"os/exec"
	"path/filepath"
)

// CopyDir copies a directory tree using cp -a.
// Returns true if the copy was performed.
func CopyDir(src, dst string) error {
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return nil
	}
	return exec.Command("cp", "-a", src, dst).Run()
}

// LockFilesMatch compares two lock files and returns true if identical.
func LockFilesMatch(fileA, fileB string) bool {
	a, errA := os.ReadFile(fileA)
	b, errB := os.ReadFile(fileB)
	if errA != nil || errB != nil {
		return false
	}
	return string(a) == string(b)
}

// CopyDeps copies vendor/ and node_modules/ from main to target.
// Returns (needComposer, needNpm) indicating if install is required.
func CopyDeps(mainDir, targetDir string) (needComposer, needNpm bool) {
	vendorSrc := filepath.Join(mainDir, "vendor")
	vendorDst := filepath.Join(targetDir, "vendor")

	if _, err := os.Stat(vendorSrc); err == nil {
		if err := CopyDir(vendorSrc, vendorDst); err == nil {
			composerLockMain := filepath.Join(mainDir, "composer.lock")
			composerLockTarget := filepath.Join(targetDir, "composer.lock")
			if !LockFilesMatch(composerLockMain, composerLockTarget) {
				needComposer = true
			}
		} else {
			needComposer = true
		}
	} else {
		needComposer = true
	}

	nodeModSrc := filepath.Join(mainDir, "node_modules")
	nodeModDst := filepath.Join(targetDir, "node_modules")

	if _, err := os.Stat(nodeModSrc); err == nil {
		if err := CopyDir(nodeModSrc, nodeModDst); err == nil {
			pkgLockMain := filepath.Join(mainDir, "package-lock.json")
			pkgLockTarget := filepath.Join(targetDir, "package-lock.json")
			if !LockFilesMatch(pkgLockMain, pkgLockTarget) {
				needNpm = true
			}
		} else {
			needNpm = true
		}
	} else if _, err := os.Stat(filepath.Join(targetDir, "package.json")); err == nil {
		needNpm = true
	}

	return needComposer, needNpm
}

// EnsureStorageDirs creates Laravel storage directory structure.
func EnsureStorageDirs(targetDir string) {
	dirs := []string{
		"storage/app/public",
		"storage/framework/cache/data",
		"storage/framework/sessions",
		"storage/framework/testing",
		"storage/framework/views",
		"storage/logs",
		"bootstrap/cache",
	}
	for _, d := range dirs {
		os.MkdirAll(filepath.Join(targetDir, d), 0777)
	}
}
