package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/H4ZM47/improved-invention/internal/app"
	taskconfig "github.com/H4ZM47/improved-invention/internal/config"
	taskdb "github.com/H4ZM47/improved-invention/internal/db"
	"github.com/H4ZM47/improved-invention/internal/report"
	"github.com/spf13/cobra"
)

func newReportCommand(opts *GlobalOptions) *cobra.Command {
	cmd := newGroupCommand("report", "Serve read-only reports")
	cmd.AddCommand(newReportServeCommand(opts))
	return cmd
}

func newReportServeCommand(opts *GlobalOptions) *cobra.Command {
	var addr string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve the local read-only HTML report server",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			cfg, db, err := openReportDB(ctx, opts)
			if err != nil {
				return err
			}
			defer db.Close()

			srv, err := report.NewServer(report.Dependencies{
				Tasks: app.TaskManager{
					DB:        db,
					HumanName: cfg.HumanName,
				},
				Links: app.LinkManager{
					DB:        db,
					HumanName: cfg.HumanName,
				},
				Relationships: app.RelationshipManager{
					DB:        db,
					HumanName: cfg.HumanName,
				},
			})
			if err != nil {
				return fmt.Errorf("build report server: %w", err)
			}

			listener, err := net.Listen("tcp", addr)
			if err != nil {
				return fmt.Errorf("start report server on %s: %w", addr, err)
			}
			defer listener.Close()

			server := &http.Server{
				Handler:           srv.Handler(),
				ReadHeaderTimeout: 5 * time.Second,
			}

			done := make(chan struct{})
			go func() {
				select {
				case <-ctx.Done():
					shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					_ = server.Shutdown(shutdownCtx)
				case <-done:
				}
			}()

			url := reportURL(listener.Addr())
			if opts.JSON {
				if err := writeJSON(cmd, map[string]any{
					"ok":      true,
					"command": "task report serve",
					"data": map[string]any{
						"url":  url,
						"addr": listener.Addr().String(),
					},
					"meta": map[string]any{},
				}); err != nil {
					close(done)
					return err
				}
			} else if !opts.Quiet {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Serving read-only report at %s\n", url); err != nil {
					close(done)
					return err
				}
			}

			err = server.Serve(listener)
			close(done)
			if err == nil || errors.Is(err, http.ErrServerClosed) || errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return nil
			}
			return fmt.Errorf("serve report server: %w", err)
		},
	}

	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:8080", "Bind address for the local report server")
	return cmd
}

func openReportDB(ctx context.Context, opts *GlobalOptions) (taskconfig.Resolved, *sql.DB, error) {
	cfg, err := taskconfig.Resolve(taskconfig.Options{
		DBPathOverride: opts.DBPath,
		ActorOverride:  opts.Actor,
	})
	if err != nil {
		return taskconfig.Resolved{}, nil, err
	}
	db, err := taskdb.Open(ctx, cfg)
	if err != nil {
		return taskconfig.Resolved{}, nil, err
	}
	return cfg, db, nil
}

func reportURL(addr net.Addr) string {
	host, port, err := net.SplitHostPort(addr.String())
	if err != nil {
		return "http://" + addr.String()
	}

	switch host {
	case "", "0.0.0.0", "::":
		host = "127.0.0.1"
	}
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}
	return "http://" + net.JoinHostPort(host, port)
}
