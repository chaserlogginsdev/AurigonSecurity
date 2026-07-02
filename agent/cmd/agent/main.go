package main

import (
	"io"
	"log"
	"os"
	"path/filepath"

	"aurigon-agent/internal/service"
)

func main() {
	// Set up logging to both stdout and a log file
	logDir := filepath.Dir(os.Args[0])
	logPath := filepath.Join(logDir, "aurigon-agent.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Warning: could not open log file %s: %v\n", logPath, err)
	} else {
		defer logFile.Close()
		log.SetOutput(io.MultiWriter(os.Stdout, logFile))
		log.Printf("Logging to %s\n", logPath)
	}

	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	if err := service.Run(); err != nil {
		log.Fatalf("agent failed: %v", err)
	}
}