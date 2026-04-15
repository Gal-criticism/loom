package main

import (
    "log"
    "os"

    "github.com/loom/daemon/cmd"
    "github.com/loom/daemon/config"
)

func main() {
    cfg, err := config.Load("config.yaml")
    if err != nil {
        log.Fatalf("Failed to load config: %v", err)
    }

    if err := cmd.Root.Execute(); err != nil {
        os.Exit(1)
    }

    _ = cfg
}
