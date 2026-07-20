package main

import (
	"bufio"
	"log"
	"os"
	"strings"
)

// loadDotEnv reads KEY=VALUE pairs from a file and sets them as process
// environment variables — but only for keys not already set. A real
// environment variable (set via NSSM, the shell, etc.) always wins over
// the .env file; this is a fallback default, never a silent override.
// Missing the file entirely is not an error.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	loaded := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)

		if key == "" {
			continue
		}
		if _, alreadySet := os.LookupEnv(key); alreadySet {
			continue
		}
		os.Setenv(key, value)
		loaded++
	}

	if loaded > 0 {
		log.Printf("Loaded %d value(s) from %s", loaded, path)
	}
}