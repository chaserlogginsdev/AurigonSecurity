package main

import (
	"bufio"
	"log"
	"os"
	"strings"
)

// loadDotEnv reads KEY=VALUE pairs from a file and sets them as process
// environment variables — but only for keys not already set. This means:
//
//   - A real environment variable (set by NSSM, the shell, Docker, etc.)
//     always wins over the .env file. The .env file is a fallback default,
//     never a silent override.
//   - Missing the file entirely is not an error — plenty of environments
//     (CI, containers) set real env vars directly and have no .env file.
//
// Lines starting with # are comments. Blank lines are skipped. Values are
// not quote-aware beyond simple leading/trailing whitespace trimming —
// keep secrets free of embedded newlines and you're fine.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		// No .env file — not an error, just means secrets come from
		// real environment variables instead.
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
		// Strip surrounding quotes if present — convenience for values
		// copied from other tools that quote everything.
		value = strings.Trim(value, `"'`)

		if key == "" {
			continue
		}
		if _, alreadySet := os.LookupEnv(key); alreadySet {
			continue // real env var takes priority — never override it
		}
		os.Setenv(key, value)
		loaded++
	}

	if loaded > 0 {
		log.Printf("Loaded %d value(s) from %s", loaded, path)
	}
}