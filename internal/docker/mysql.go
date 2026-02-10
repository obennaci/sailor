package docker

import (
	"fmt"
	"os/exec"
	"strings"
)

// FindMySQLContainer detects the MySQL container name from docker compose ps.
func FindMySQLContainer(mainDir string) (string, error) {
	containers, err := ComposePS(mainDir)
	if err != nil {
		return "", fmt.Errorf("failed to list containers: %w", err)
	}

	for _, c := range containers {
		svc := strings.ToLower(c.Service)
		if svc == "mysql" || svc == "mariadb" || svc == "db" {
			return c.Name, nil
		}
	}

	// Fallback: guess from directory name
	return "", fmt.Errorf("no MySQL container found — is the main branch running?")
}

// MySQLIsReachable checks if the MySQL container is responding to pings.
func MySQLIsReachable(container string) bool {
	if container == "" {
		return false
	}
	err := exec.Command("docker", "exec", container, "mysqladmin", "ping", "--silent").Run()
	return err == nil
}

// MySQLExec runs a MySQL command.
func MySQLExec(container, password, sql string) error {
	args := []string{"exec", container, "mysql", "-u", "root", "-p" + password, "-e", sql}
	cmd := exec.Command("docker", args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

// MySQLCreateDB creates a database if it doesn't exist.
func MySQLCreateDB(container, password, dbName string) error {
	sql := fmt.Sprintf(
		"CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;",
		dbName,
	)
	if err := MySQLExec(container, password, sql); err != nil {
		return err
	}
	grant := fmt.Sprintf("GRANT ALL PRIVILEGES ON `%s`.* TO 'root'@'%%';", dbName)
	MySQLExec(container, password, grant) // best-effort
	return nil
}

// MySQLDropDB drops a database.
func MySQLDropDB(container, password, dbName string) error {
	sql := fmt.Sprintf("DROP DATABASE IF EXISTS `%s`;", dbName)
	return MySQLExec(container, password, sql)
}

// MySQLHasTables checks if a database has any tables.
func MySQLHasTables(container, password, dbName string) bool {
	args := []string{"exec", container, "mysql", "-u", "root", "-p" + password, "-e",
		fmt.Sprintf("USE `%s`; SHOW TABLES;", dbName)}
	out, err := exec.Command("docker", args...).Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != ""
}

// MySQLDump dumps a database (schema only or full).
func MySQLDump(container, password, dbName string, schemaOnly bool) (string, error) {
	args := []string{"exec", container, "mysqldump", "-u", "root", "-p" + password}
	if schemaOnly {
		args = append(args, "--no-data")
	}
	args = append(args, dbName)
	out, err := exec.Command("docker", args...).Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// MySQLImport imports SQL into a database.
func MySQLImport(container, password, dbName, sql string) error {
	return ExecStdin(container, sql, "mysql", "-u", "root", "-p"+password, dbName)
}
