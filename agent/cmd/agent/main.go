package main

import (
	"io"
	"log"
	"os"
	"path/filepath"

	"aurigon-agent/internal/service"
)

func main() {
	exeDir := filepath.Dir(os.Args[0])
	logPath := filepath.Join(exeDir, "aurigon-agent.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Warning: could not open log file: %v", err)
	} else {
		defer logFile.Close()
		log.SetOutput(io.MultiWriter(os.Stdout, logFile))
	}
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	stop := make(chan struct{})
	if err := service.RunWithStop(stop); err != nil {
		log.Fatalf("agent failed: %v", err)
	}
}