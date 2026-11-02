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

	"github.com/dbohry/huginn/internal/auth"
	"github.com/dbohry/huginn/internal/config"
	"github.com/dbohry/huginn/internal/db"
	"github.com/dbohry/huginn/internal/server"
	"github.com/dbohry/huginn/internal/store"
	"github.com/dbohry/huginn/internal/webui"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

const usage = `Usage: huginn [command]

Commands:
  serve                    Run the server (default)
  reset-password <user>    Give a user a temporary password and sign them out
  healthcheck              Exit 0 if a running server is healthy, 1 if not
  version                  Print the version

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
	case "reset-password":
		err = resetPassword(config.Load(), os.Args[2:])
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
	go purgeSessions(ctx, store.New(conn), log)

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           server.Start(ctx, conn, webui.Assets(), version),
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

// purgeSessions deletes expired sessions once at start-up and then hourly.
func purgeSessions(ctx context.Context, st *store.Store, log *slog.Logger) {
	run := func() {
		n, err := st.PurgeExpiredSessions(ctx, time.Now())
		switch {
		case err != nil && ctx.Err() == nil:
			log.Error("purge expired sessions", "err", err)
		case n > 0:
			log.Info("purged expired sessions", "count", n)
		}
	}

	run()
	tick := time.NewTicker(time.Hour)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			run()
		}
	}
}

// resetPassword is the way back in for someone who is locked out. It runs on
// the machine that hosts Huginn, so it needs access to the data directory.
func resetPassword(cfg config.Config, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: huginn reset-password <username>")
	}
	username := args[0]

	// Do not create an empty database by accident if the data directory is wrong.
	if _, err := os.Stat(cfg.DBPath()); err != nil {
		return fmt.Errorf("no Huginn database at %s (is HUGINN_DATA_DIR right?)", cfg.DBPath())
	}
	conn, err := db.Open(cfg.DBPath())
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := db.Migrate(conn); err != nil {
		return err
	}

	temp, err := auth.ResetPassword(context.Background(), store.New(conn), username)
	if errors.Is(err, store.ErrNotFound) {
		return fmt.Errorf("there is no user named %q", username)
	}
	if err != nil {
		return err
	}

	fmt.Printf("Temporary password for %s: %s\n\n", username, temp)
	fmt.Println("They have been signed out everywhere and must choose a new password the next time they sign in.")
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
