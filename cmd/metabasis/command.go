package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/woodleighschool/metabasis/internal/app"
	"github.com/woodleighschool/metabasis/internal/config"
	"github.com/woodleighschool/metabasis/internal/httpapi"
	"github.com/woodleighschool/metabasis/internal/intent"
	"github.com/woodleighschool/metabasis/internal/metrics"
	"github.com/woodleighschool/metabasis/internal/reconcile"
)

func newRootCommand() *cobra.Command {
	var configPaths []string
	command := &cobra.Command{
		Use:           "metabasis",
		Short:         "Reconcile temporary Entra group membership",
		Version:       version,
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return command.Help()
		},
	}
	command.PersistentFlags().StringArrayVar(
		&configPaths,
		"config",
		defaultConfigPaths(),
		"path to a YAML configuration file; may be repeated in overlay order",
	)
	command.AddCommand(
		newValidateCommand(&configPaths),
		newPlanCommand(&configPaths),
		newRunCommand(&configPaths),
		newIntentsCommand(&configPaths),
		newReconcileCommand(&configPaths),
		newSchemaCommand(),
		newVersionCommand(),
	)
	return command
}

func defaultConfigPaths() []string {
	info, err := os.Stat("config.yaml")
	if err != nil || !info.Mode().IsRegular() {
		return nil
	}
	return []string{"config.yaml"}
}

func newValidateCommand(configPaths *[]string) *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration and identity expressions",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if _, err := config.Load((*configPaths)...); err != nil {
				return fmt.Errorf("validate configuration: %w", err)
			}
			_, err := fmt.Fprintln(command.OutOrStdout(), "configuration valid")
			return err
		},
	}
}

func newPlanCommand(configPaths *[]string) *cobra.Command {
	var eventPath string
	var output string
	command := &cobra.Command{
		Use:   "plan",
		Short: "Print the read-only identity plan for a canonical event",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if eventPath == "" {
				return fmt.Errorf("--event is required")
			}
			if output != "human" && output != "json" {
				return fmt.Errorf("output must be human or json")
			}
			cfg, err := config.Load((*configPaths)...)
			if err != nil {
				return fmt.Errorf("load configuration: %w", err)
			}
			event, err := readEvent(eventPath)
			if err != nil {
				return err
			}
			event.Source, err = soleWebhookSource(cfg)
			if err != nil {
				return err
			}
			application, err := app.Build(command.Context(), cfg, false, nil)
			if err != nil {
				return fmt.Errorf("start metabasis read-only: %w", err)
			}
			plan, planErr := application.Reconciler.PlanEvent(command.Context(), event)
			writeErr := writePlan(command.OutOrStdout(), output, plan)
			application.Close()
			return errors.Join(planErr, writeErr)
		},
	}
	command.Flags().StringVar(&eventPath, "event", "", "path to a canonical intent JSON file")
	command.Flags().StringVar(&output, "output", "human", "plan output format: human or json")
	return command
}

func newRunCommand(configPaths *[]string) *cobra.Command {
	var logLevel string
	command := &cobra.Command{
		Use:   "run",
		Short: "Serve webhooks and reconcile scheduled identity state",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			level, err := parseLogLevel(logLevel)
			if err != nil {
				return err
			}
			cfg, err := config.Load((*configPaths)...)
			if err != nil {
				return fmt.Errorf("load configuration: %w", err)
			}
			logger := slog.New(slog.NewJSONHandler(command.ErrOrStderr(), &slog.HandlerOptions{Level: level}))
			wake := make(chan struct{}, 1)
			recorder := metrics.New(metrics.BuildInfo{Version: version, Revision: commit}, logger)
			application, err := app.Build(command.Context(), cfg, true, recorder)
			if err != nil {
				return fmt.Errorf("start metabasis: %w", err)
			}
			defer application.Close()
			handler := httpapi.NewHandler(cfg, application.Store, wake, logger, recorder)
			metricsMux := http.NewServeMux()
			metricsMux.Handle("GET /metrics", recorder.Handler())
			return runService(command.Context(), cfg, application, handler, metricsMux, wake, logger)
		},
	}
	command.Flags().StringVar(&logLevel, "log-level", "info", "log level: debug, info, warn, or error")
	return command
}

func runService(
	parent context.Context,
	cfg *config.Config,
	application *app.App,
	handler http.Handler,
	metricsHandler http.Handler,
	wake <-chan struct{},
	logger *slog.Logger,
) error {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	servers := []*http.Server{
		{
			Addr:              cfg.Listen,
			Handler:           handler,
			ReadHeaderTimeout: 5 * time.Second,
			IdleTimeout:       time.Minute,
		},
		{
			Addr:              cfg.MetricsListen,
			Handler:           metricsHandler,
			ReadHeaderTimeout: 5 * time.Second,
			IdleTimeout:       time.Minute,
		},
	}
	serverErrors := make(chan error, len(servers))
	for _, server := range servers {
		go func() {
			serverErrors <- server.ListenAndServe()
		}()
	}
	reconcilerDone := make(chan struct{})
	go func() {
		reconcile.RunLoop(ctx, application.Reconciler, wake, cfg.Reconcile.PollInterval.Duration, logger)
		close(reconcilerDone)
	}()
	logger.InfoContext(ctx, "Metabasis started", "version", version, "listen", cfg.Listen, "metrics_listen", cfg.MetricsListen)

	var runErr error
	select {
	case <-parent.Done():
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			runErr = fmt.Errorf("serve HTTP: %w", err)
		}
	}
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.WithoutCancel(parent), 10*time.Second)
	defer shutdownCancel()
	for _, server := range servers {
		if err := server.Shutdown(shutdownCtx); err != nil {
			runErr = errors.Join(runErr, fmt.Errorf("shutdown HTTP server on %s: %w", server.Addr, err))
		}
	}
	select {
	case <-reconcilerDone:
	case <-shutdownCtx.Done():
		runErr = errors.Join(runErr, fmt.Errorf("stop reconciler: %w", shutdownCtx.Err()))
	}
	return runErr
}

func newIntentsCommand(configPaths *[]string) *cobra.Command {
	command := &cobra.Command{Use: "intents", Short: "Inspect accepted intents", Args: cobra.NoArgs}
	command.AddCommand(newIntentsListCommand(configPaths), newIntentsShowCommand(configPaths))
	return command
}

func newIntentsListCommand(configPaths *[]string) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List accepted intents",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			application, err := buildOperationalApp(command.Context(), *configPaths)
			if err != nil {
				return err
			}
			defer application.Close()
			intents, err := application.Store.ListAllIntents(command.Context())
			if err != nil {
				return err
			}
			return writeIntents(command.OutOrStdout(), intents, time.Now().UTC())
		},
	}
}

func newIntentsShowCommand(configPaths *[]string) *cobra.Command {
	return &cobra.Command{
		Use:   "show <source> <id>",
		Short: "Show one accepted intent and its subject reconciliation state",
		Args:  cobra.ExactArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			application, err := buildOperationalApp(command.Context(), *configPaths)
			if err != nil {
				return err
			}
			defer application.Close()
			accepted, err := application.Store.GetIntent(command.Context(), args[0], args[1])
			if err != nil {
				return err
			}
			state, err := application.Store.GetState(command.Context(), accepted.Subject)
			if err != nil {
				return err
			}
			return writeJSON(command.OutOrStdout(), map[string]any{
				"intent": accepted,
				"phase":  accepted.PhaseAt(time.Now().UTC()),
				"state":  state,
			})
		},
	}
}

func newReconcileCommand(configPaths *[]string) *cobra.Command {
	var subject string
	var all bool
	command := &cobra.Command{
		Use:   "reconcile",
		Short: "Reconcile one subject or all accepted subjects now",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if (strings.TrimSpace(subject) == "") == !all {
				return fmt.Errorf("set exactly one of --subject or --all")
			}
			application, err := buildOperationalApp(command.Context(), *configPaths)
			if err != nil {
				return err
			}
			defer application.Close()
			var results []reconcile.Result
			if all {
				results, err = application.Reconciler.ReconcileAll(command.Context())
			} else {
				var result reconcile.Result
				result, err = application.Reconciler.ReconcileSubject(command.Context(), strings.TrimSpace(subject))
				results = []reconcile.Result{result}
			}
			return errors.Join(writeReconcileResults(command.OutOrStdout(), results), err)
		},
	}
	command.Flags().StringVar(&subject, "subject", "", "subject to reconcile")
	command.Flags().BoolVar(&all, "all", false, "reconcile all subjects")
	return command
}

func buildOperationalApp(ctx context.Context, configPaths []string) (*app.App, error) {
	cfg, err := config.Load(configPaths...)
	if err != nil {
		return nil, fmt.Errorf("load configuration: %w", err)
	}
	application, err := app.Build(ctx, cfg, true, nil)
	if err != nil {
		return nil, fmt.Errorf("start metabasis: %w", err)
	}
	return application, nil
}

func newSchemaCommand() *cobra.Command {
	var outputPath string
	command := &cobra.Command{
		Use:   "schema",
		Short: "Generate the JSON Schema used by YAML editors",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			document, err := config.JSONSchemaDocument()
			if err != nil {
				return fmt.Errorf("generate config schema: %w", err)
			}
			if outputPath == "-" {
				_, err = command.OutOrStdout().Write(document)
				return err
			}
			if err := os.WriteFile(outputPath, document, 0o644); err != nil {
				return fmt.Errorf("write config schema: %w", err)
			}
			return nil
		},
	}
	command.Flags().StringVar(&outputPath, "output", "-", "schema output path, or - for stdout")
	return command
}

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(command.OutOrStdout(), "metabasis %s\ncommit: %s\nbuilt: %s\n", version, commit, date)
			return err
		},
	}
}

func readEvent(path string) (intent.Intent, error) {
	file, err := os.Open(path)
	if err != nil {
		return intent.Intent{}, fmt.Errorf("open event: %w", err)
	}
	defer func() { _ = file.Close() }()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var event intent.Intent
	if err := decoder.Decode(&event); err != nil {
		return intent.Intent{}, fmt.Errorf("decode event: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return intent.Intent{}, fmt.Errorf("event must contain one JSON object")
	}
	return event, nil
}

func soleWebhookSource(cfg *config.Config) (string, error) {
	if len(cfg.Webhooks) != 1 {
		return "", fmt.Errorf("plan requires exactly one configured webhook source")
	}
	for source := range cfg.Webhooks {
		return source, nil
	}
	return "", fmt.Errorf("plan requires a configured webhook source")
}

func parseLogLevel(value string) (slog.Level, error) {
	switch strings.ToLower(value) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("log level must be debug, info, warn, or error")
	}
}
