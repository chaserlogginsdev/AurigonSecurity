package main

import (
    "log"
    "aurigon-agent/internal/service"
)

func main() {
    if err := service.Run(); err != nil {
        log.Fatalf("agent failed: %v", err)
    }
}
