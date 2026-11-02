// Command huginn runs the Huginn RSS reader and its operations subcommands.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dbohry/huginn/internal/config"
	"github.com/dbohry/huginn/internal/db"
	"github.com/dbohry/huginn/internal/server"
	"github.com/dbohry/huginn/internal/webui"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

const usage = `Usage: huginn [command]

Commands:
  serve          Run the server (default)
  healthcheck    Exit 0 if a running server is healthy, 1 if not
  version        Print the version

Environment:
  HUGINN_ADDR       Listen address (default ":8080")
  HUGINN_DATA_DIR   Directory for the database (default "/data")
`

func main() {
	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	var err error
	switch cmd {
	case "serve":
		err = serve(config.Load())
	case "healthcheck":
		err = healthcheck(config.Load())
	case "version":
		fmt.Println(version)
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", cmd, usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func serve(cfg config.Config) error {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))

	if err := os.MkdirAll(cfg.DataDir, 0o750); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}
	conn, err := db.Open(cfg.DBPath())
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := db.Migrate(conn); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           server.New(conn, webui.Assets(), version),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	log.Info("listening", "addr", cfg.Addr, "data", cfg.DataDir, "version", version)

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}

	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// healthcheck asks the local server for /healthz. It exists because the
// container image has no curl or shell.
func healthcheck(cfg config.Config) error {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(cfg.HealthURL())
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unhealthy: %s", resp.Status)
	}
	return nil
}
