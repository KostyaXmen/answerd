package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	cmd := "run"
	args := os.Args[1:]

	if len(args) > 0 {
		switch args[0] {
		case "run", "screenshot":
			cmd = args[0]
			args = args[1:]
		default:
			if len(args[0]) == 0 || args[0][0] != '-' {
				fmt.Fprintf(os.Stderr, "unknown command: %s\n", args[0])
				os.Exit(2)
			}
			// Otherwise it is a flag for the default "run" command.
		}
	}

	fs := flag.NewFlagSet("answerd", flag.ExitOnError)
	configPath := fs.String("config", "", "path to config.json")
	_ = fs.Parse(args)

	if cmd == "screenshot" {
		cfg, err := LoadConfigPaths(*configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

		logScreenshot("capturing")
		if err := captureToInbox(context.Background(), cfg); err != nil {
			logScreenshot("error: " + err.Error())
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		logScreenshot("saved to inbox")
		return
	}

	cfg, err := LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	setupLogging(cfg.LogFile)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := RunDaemon(ctx, cfg); err != nil {
		log.Printf("fatal: %v", err)
		os.Exit(1)
	}
}

// setupLogging mirrors log output to stderr and to a file (if configured).
func setupLogging(path string) {
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		log.Printf("cannot create log dir: %v", err)
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		log.Printf("cannot open log file %s: %v", path, err)
		return
	}
	log.SetOutput(io.MultiWriter(os.Stderr, f))
	log.Printf("logging to %s", path)
}

// logScreenshot appends a line to the standard log file. Used by the
// lightweight screenshot subcommand, which does not need an API key.
func logScreenshot(msg string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	path := filepath.Join(home, ".cache", "answerd", "answerd.log")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s screenshot: %s\n", time.Now().Format(time.RFC3339), msg)
}
