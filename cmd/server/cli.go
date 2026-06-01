package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/alvindcastro/groupscout/config"
	"github.com/alvindcastro/groupscout/internal/auditretention"
	"github.com/alvindcastro/groupscout/internal/logger"
	"github.com/alvindcastro/groupscout/internal/ollama"
	"github.com/alvindcastro/groupscout/internal/storage"
)

func handleOllamaCommands(cfg *config.Config) {
	if flag.NArg() < 2 {
		fmt.Println("Usage: ollama [push-models | list-models]")
		os.Exit(1)
	}

	oc := &ollama.OllamaClient{
		Endpoint: cfg.OllamaEndpoint,
		Model:    cfg.OllamaModel,
		Timeout:  30 * time.Second,
	}
	manager := ollama.NewModelfileManager(oc)
	ctx := context.Background()

	switch flag.Arg(1) {
	case "push-models":
		pushModels(ctx, manager)
	case "list-models":
		listModels(ctx, manager)
	default:
		fmt.Printf("Unknown ollama subcommand: %s\n", flag.Arg(1))
		os.Exit(1)
	}
	os.Exit(0)
}

func pushModels(ctx context.Context, manager *ollama.ModelfileManager) {
	files, err := os.ReadDir("internal/ollama/modelfile")
	if err != nil {
		log.Fatalf("failed to read modelfiles: %v", err)
	}

	for _, f := range files {
		if !strings.HasSuffix(f.Name(), ".modelfile") {
			continue
		}

		path := filepath.Join("internal/ollama/modelfile", f.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			log.Printf("failed to read %s: %v", path, err)
			continue
		}

		stem := strings.TrimSuffix(f.Name(), ".modelfile")
		modelName := "groupscout-" + strings.ReplaceAll(stem, "_", "-")

		fmt.Printf("Pushing %s as %s... ", f.Name(), modelName)
		if err := manager.Push(ctx, modelName, string(content)); err != nil {
			fmt.Printf("FAILED: %v\n", err)
		} else {
			fmt.Println("OK")
		}
	}
}

func listModels(ctx context.Context, manager *ollama.ModelfileManager) {
	models, err := manager.ListModels(ctx)
	if err != nil {
		log.Fatalf("failed to list models: %v", err)
	}

	fmt.Println("Loaded Ollama models:")
	for _, m := range models {
		fmt.Printf("- %s\n", m)
	}
}

func handleAuditCommand(cfg *config.Config) {
	auditCmd := flag.NewFlagSet("audit", flag.ExitOnError)
	savePath := auditCmd.String("save", "", "save payload to a file")
	showMeta := auditCmd.Bool("meta", false, "show metadata only")
	auditCmd.Parse(flag.Args()[1:])

	if auditCmd.NArg() < 1 {
		fmt.Println("Usage: groupscout audit <lead_id> [--save <path>] [--meta]")
		os.Exit(1)
	}
	leadID := auditCmd.Arg(0)

	db, err := storage.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	leadStore := storage.NewLeadStoreWithDSN(db, cfg.DatabaseURL)
	lead, err := leadStore.GetByID(ctx, leadID)
	if err != nil {
		log.Fatalf("get lead: %v", err)
	}
	if lead == nil {
		log.Fatalf("lead %s not found", leadID)
	}

	if lead.RawInputID == "" {
		log.Fatalf("lead %s has no raw input associated", leadID)
	}

	rawInputID, err := uuid.Parse(lead.RawInputID)
	if err != nil {
		log.Fatalf("invalid raw input ID: %v", err)
	}

	auditStore := storage.NewAuditStoreWithDSN(db, cfg.DatabaseURL)
	raw, err := auditStore.GetByID(ctx, rawInputID)
	if err != nil {
		log.Fatalf("get audit record: %v", err)
	}
	if raw == nil {
		log.Fatalf("raw input %s not found", lead.RawInputID)
	}

	if *showMeta {
		fmt.Printf("Lead:         %s\n", lead.Title)
		fmt.Printf("Audit ID:     %s\n", raw.ID)
		fmt.Printf("Source URL:   %s\n", raw.SourceURL)
		fmt.Printf("Collector:    %s\n", raw.CollectorName)
		fmt.Printf("Payload Type: %s\n", raw.PayloadType)
		fmt.Printf("Fetched At:   %s\n", raw.CreatedAt.Format(time.RFC3339))
		fmt.Printf("Hash:         %s\n", raw.Hash)
		return
	}

	if *savePath != "" {
		if err := os.WriteFile(*savePath, raw.Payload, 0644); err != nil {
			log.Fatalf("save file: %v", err)
		}
		fmt.Printf("Saved payload to %s\n", *savePath)
	} else {
		os.Stdout.Write(raw.Payload)
		fmt.Println()
	}
}

func handleAuditRetentionCommand(cfg *config.Config) {
	retentionCmd := flag.NewFlagSet("audit-retention", flag.ExitOnError)
	days := retentionCmd.Int("days", cfg.AuditRetentionDays, "delete unreferenced raw inputs older than this many days")

	args := flag.Args()[1:]
	if len(args) < 1 || args[0] != "purge" {
		fmt.Println("Usage: groupscout audit-retention purge [--days N]")
		os.Exit(1)
	}
	retentionCmd.Parse(args[1:])

	db, err := storage.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()

	if err := storage.Migrate(db, cfg.DatabaseURL); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	worker, err := newAuditRetentionWorker(cfg, db, auditretention.Policy{
		RetentionDays: *days,
		Interval:      24 * time.Hour,
	})
	if err != nil {
		log.Fatalf("audit retention config: %v", err)
	}

	result, err := worker.PurgeOnce(context.Background())
	if err != nil {
		log.Fatalf("audit retention purge: %v", err)
	}

	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		log.Fatalf("encode result: %v", err)
	}
}

func startAuditRetentionWorker(cfg *config.Config, db *sql.DB) {
	if !cfg.AuditRetentionEnabled {
		return
	}

	worker, err := newAuditRetentionWorker(cfg, db, auditretention.Policy{
		RetentionDays: cfg.AuditRetentionDays,
		Interval:      time.Duration(cfg.AuditRetentionIntervalH) * time.Hour,
		RunOnStart:    cfg.AuditRetentionRunOnStart,
	})
	if err != nil {
		logger.Log.Error("invalid audit retention configuration", "error", err)
		os.Exit(1)
	}

	go worker.Run(context.Background())
	logger.Log.Info(
		"audit retention worker started",
		"retention_days", cfg.AuditRetentionDays,
		"interval_hours", cfg.AuditRetentionIntervalH,
		"run_on_start", cfg.AuditRetentionRunOnStart,
	)
}

func newAuditRetentionWorker(cfg *config.Config, db *sql.DB, policy auditretention.Policy) (*auditretention.Worker, error) {
	return auditretention.New(
		storage.NewAuditStoreWithDSN(db, cfg.DatabaseURL),
		policy,
		logger.Log,
	)
}
