package env

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Read parses a .env file into a map.
func Read(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	env := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, "=")
		if idx < 0 {
			continue
		}
		key := line[:idx]
		val := line[idx+1:]
		val = strings.Trim(val, `"'`)
		env[key] = val
	}
	return env, scanner.Err()
}

// Get reads a single value from a .env file, returning defaultVal if not found.
func Get(path, key, defaultVal string) string {
	env, err := Read(path)
	if err != nil {
		return defaultVal
	}
	if v, ok := env[key]; ok && v != "" {
		return v
	}
	return defaultVal
}

// Write updates or appends key=value pairs in a .env file,
// preserving existing lines and comments.
func Write(path string, updates map[string]string) error {
	var lines []string
	updated := make(map[string]bool)

	if f, err := os.Open(path); err == nil {
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			trimmed := strings.TrimSpace(line)

			if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
				idx := strings.Index(trimmed, "=")
				if idx > 0 {
					key := trimmed[:idx]
					if newVal, ok := updates[key]; ok {
						lines = append(lines, fmt.Sprintf("%s=%s", key, newVal))
						updated[key] = true
						continue
					}
				}
			}
			lines = append(lines, line)
		}
		f.Close()
	}

	// Append keys not yet present
	for key, val := range updates {
		if !updated[key] {
			lines = append(lines, fmt.Sprintf("%s=%s", key, val))
		}
	}

	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0644)
}

// Copy copies a .env file from src to dst.
func Copy(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}
