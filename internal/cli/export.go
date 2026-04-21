package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/H4ZM47/task-cli/internal/app"
	taskconfig "github.com/H4ZM47/task-cli/internal/config"
	taskdb "github.com/H4ZM47/task-cli/internal/db"
	"github.com/H4ZM47/task-cli/internal/export"
	"github.com/spf13/cobra"
)

func newExportCommand(opts *GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export task data",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(
		newExportFormatCommand(opts, "json", "Export task data as JSON", export.EncodeJSON),
		newExportFormatCommand(opts, "csv", "Export task data as CSV", export.EncodeCSV),
		newExportFormatCommand(opts, "txt", "Export task data as plain text", encodeTXTBundle),
		newExportFormatCommand(opts, "markdown", "Export task data as Markdown", encodeMarkdownBundle),
	)
	return cmd
}

type bundleEncoder func(export.Bundle) ([]byte, error)

func encodeTXTBundle(b export.Bundle) ([]byte, error)      { return export.EncodeTXT(b), nil }
func encodeMarkdownBundle(b export.Bundle) ([]byte, error) { return export.EncodeMarkdown(b), nil }

func newExportFormatCommand(opts *GlobalOptions, use, short string, encode bundleEncoder) *cobra.Command {
	var output string

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			db, err := openExportDB(ctx, opts)
			if err != nil {
				return err
			}
			defer db.Close()

			bundle, err := buildExportBundle(ctx, db)
			if err != nil {
				return err
			}

			payload, err := encode(bundle)
			if err != nil {
				return err
			}

			return writeExportOutput(cmd, output, payload)
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "", "Write export to a file path instead of stdout")

	return cmd
}

func openExportDB(ctx context.Context, opts *GlobalOptions) (*sql.DB, error) {
	cfg, err := taskconfig.Resolve(taskconfig.Options{
		DBPathOverride: opts.DBPath,
		ActorOverride:  opts.Actor,
	})
	if err != nil {
		return nil, err
	}
	return taskdb.Open(ctx, cfg)
}

func buildExportBundle(ctx context.Context, db *sql.DB) (export.Bundle, error) {
	tasks, err := app.TaskManager{DB: db}.List(ctx, app.ListTasksRequest{})
	if err != nil {
		return export.Bundle{}, fmt.Errorf("list tasks: %w", err)
	}
	domains, err := app.DomainManager{DB: db}.List(ctx, app.ListDomainsRequest{})
	if err != nil {
		return export.Bundle{}, fmt.Errorf("list domains: %w", err)
	}
	projects, err := app.ProjectManager{DB: db}.List(ctx, app.ListProjectsRequest{})
	if err != nil {
		return export.Bundle{}, fmt.Errorf("list projects: %w", err)
	}
	actors, err := app.ActorManager{DB: db}.List(ctx, app.ListActorsRequest{})
	if err != nil {
		return export.Bundle{}, fmt.Errorf("list actors: %w", err)
	}

	linkMgr := app.LinkManager{DB: db}
	relMgr := app.RelationshipManager{DB: db}
	var links []app.LinkRecord
	var relationships []app.RelationshipRecord
	seenRelationships := make(map[string]struct{})

	for _, task := range tasks {
		taskLinks, err := linkMgr.List(ctx, app.ListLinksRequest{TaskRef: task.Handle})
		if err != nil {
			return export.Bundle{}, fmt.Errorf("list links for task %s: %w", task.Handle, err)
		}
		links = append(links, taskLinks...)

		taskRels, err := relMgr.List(ctx, app.ListRelationshipsRequest{TaskRef: task.Handle})
		if err != nil {
			return export.Bundle{}, fmt.Errorf("list relationships for task %s: %w", task.Handle, err)
		}
		for _, rel := range taskRels {
			if _, ok := seenRelationships[rel.UUID]; ok {
				continue
			}
			seenRelationships[rel.UUID] = struct{}{}
			relationships = append(relationships, rel)
		}
	}

	return export.Bundle{
		Tasks:         tasks,
		Domains:       domains,
		Projects:      projects,
		Actors:        actors,
		Links:         links,
		Relationships: relationships,
	}, nil
}

func writeExportOutput(cmd *cobra.Command, path string, payload []byte) error {
	if path == "" {
		_, err := cmd.OutOrStdout().Write(payload)
		return err
	}
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		return fmt.Errorf("write export to %s: %w", path, err)
	}
	return nil
}
