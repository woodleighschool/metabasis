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

func newRootCommand() (*cobra.Command, *commandOutput) {
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
	output := newCommandOutput(command)
	command.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		return output.start(cmd)
	}
	command.PersistentFlags().StringArrayVar(
		&configPaths,
		"config",
		defaultConfigPaths(),
		"path to a YAML configuration file; may be repeated in overlay order",
	)
	command.AddCommand(
		newValidateCommand(&configPaths, output),
		newPlanCommand(&configPaths, output),
		newRunCommand(&configPaths, output),
		newIntentsCommand(&configPaths, output),
		newApplyCommand(&configPaths, output),
		newSchemaCommand(),
		newVersionCommand(),
	)
	return command, output
}

func defaultConfigPaths() []string {
	info, err := os.Stat("config.yaml")
	if err != nil || !info.Mode().IsRegular() {
		return nil
	}
	return []string{"config.yaml"}
}

func newValidateCommand(configPaths *[]string, diagnostics *commandOutput) *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration and identity expressions",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if _, err := loadConfig(command, *configPaths, diagnostics); err != nil {
				return fmt.Errorf("validate configuration: %w", err)
			}
			_, err := fmt.Fprintln(command.OutOrStdout(), "configuration valid")
			return err
		},
	}
}

func newPlanCommand(configPaths *[]string, diagnostics *commandOutput) *cobra.Command {
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
			if output != "text" && output != "json" {
				return fmt.Errorf("output must be text or json")
			}
			cfg, err := loadConfig(command, *configPaths, diagnostics)
			if err != nil {
				return err
			}
			event, err := readEvent(eventPath)
			if err != nil {
				return err
			}
			event.Source, err = soleWebhookSource(cfg)
			if err != nil {
				return err
			}
			application, err := app.Build(command.Context(), cfg, false, nil, diagnostics.logger)
			if err != nil {
				return fmt.Errorf("start read-only service: %w", err)
			}
			plan, planErr := application.Reconciler.PlanEvent(command.Context(), event)
			diagnostics.endProgress(planErr)
			writeErr := writePlan(command.OutOrStdout(), output, plan, planErr)
			application.Close()
			return errors.Join(planErr, writeErr)
		},
	}
	command.Flags().StringVar(&eventPath, "event", "", "path to a canonical intent JSON file")
	command.Flags().StringVar(&output, "output", "text", "Report format: text or json")
	return command
}

func newRunCommand(configPaths *[]string, diagnostics *commandOutput) *cobra.Command {
	command := &cobra.Command{
		Use:   "run",
		Short: "Serve webhooks and reconcile scheduled identity state",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			cfg, err := loadConfig(command, *configPaths, diagnostics)
			if err != nil {
				return err
			}
			logger := diagnostics.logger
			wake := make(chan struct{}, 1)
			recorder := metrics.New(metrics.BuildInfo{Version: version, Revision: commit}, logger)
			application, err := app.Build(command.Context(), cfg, true, recorder, logger)
			if err != nil {
				return fmt.Errorf("start service: %w", err)
			}
			defer application.Close()
			handler := httpapi.NewHandler(cfg, application.Store, wake, logger, recorder)
			metricsMux := http.NewServeMux()
			metricsMux.Handle("GET /metrics", recorder.Handler())
			return runService(command.Context(), cfg, application, handler, metricsMux, wake, logger)
		},
	}
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
		runLoop(ctx, cfg.Reconcile.PollInterval.Duration, application.Reconciler, wake, logger)
		close(reconcilerDone)
	}()
	logger.InfoContext(ctx, "Service started", "version", version, "listen", cfg.Listen, "metrics_listen", cfg.MetricsListen)

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

func newIntentsCommand(configPaths *[]string, diagnostics *commandOutput) *cobra.Command {
	command := &cobra.Command{Use: "intents", Short: "Inspect accepted intents", Args: cobra.NoArgs}
	command.AddCommand(newIntentsListCommand(configPaths, diagnostics), newIntentsShowCommand(configPaths, diagnostics))
	return command
}

func newIntentsListCommand(configPaths *[]string, diagnostics *commandOutput) *cobra.Command {
	var output string
	command := &cobra.Command{
		Use:   "list",
		Short: "List accepted intents",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if output != "text" && output != "json" {
				return fmt.Errorf("output must be text or json")
			}
			application, err := buildOperationalApp(command, *configPaths, diagnostics)
			if err != nil {
				return err
			}
			defer application.Close()
			intents, err := application.Store.ListAllIntents(command.Context())
			if err != nil {
				return err
			}
			if output == "json" {
				if intents == nil {
					intents = []intent.Intent{}
				}
				return writeJSON(command.OutOrStdout(), map[string]any{"intents": intents})
			}
			return writeIntents(command.OutOrStdout(), intents, time.Now().UTC())
		},
	}
	command.Flags().StringVar(&output, "output", "text", "Report format: text or json")
	return command
}

func newIntentsShowCommand(configPaths *[]string, diagnostics *commandOutput) *cobra.Command {
	var output string
	command := &cobra.Command{
		Use:   "show <source> <id>",
		Short: "Show one accepted intent and its subject reconciliation state",
		Args:  cobra.ExactArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			if output != "text" && output != "json" {
				return fmt.Errorf("output must be text or json")
			}
			application, err := buildOperationalApp(command, *configPaths, diagnostics)
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
			if output == "text" {
				return writeIntent(command.OutOrStdout(), accepted, state, time.Now().UTC())
			}
			return writeJSON(command.OutOrStdout(), map[string]any{
				"intent": accepted,
				"phase":  accepted.PhaseAt(time.Now().UTC()),
				"state":  state,
			})
		},
	}
	command.Flags().StringVar(&output, "output", "text", "Report format: text or json")
	return command
}

func newApplyCommand(configPaths *[]string, diagnostics *commandOutput) *cobra.Command {
	var subject string
	var all bool
	var output string
	command := &cobra.Command{
		Use:   "apply",
		Short: "Reconcile one subject or all accepted subjects now",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if output != "text" && output != "json" {
				return fmt.Errorf("output must be text or json")
			}
			if (strings.TrimSpace(subject) == "") == !all {
				return fmt.Errorf("set exactly one of --subject or --all")
			}
			application, err := buildOperationalApp(command, *configPaths, diagnostics)
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
			diagnostics.endProgress(err)
			return errors.Join(err, writeApplyReport(command.OutOrStdout(), output, results, err))
		},
	}
	command.Flags().StringVar(&subject, "subject", "", "subject to reconcile")
	command.Flags().BoolVar(&all, "all", false, "Reconcile all accepted subjects")
	command.Flags().StringVar(&output, "output", "text", "Report format: text or json")
	return command
}

func buildOperationalApp(command *cobra.Command, configPaths []string, diagnostics *commandOutput) (*app.App, error) {
	cfg, err := loadConfig(command, configPaths, diagnostics)
	if err != nil {
		return nil, err
	}
	application, err := app.Build(command.Context(), cfg, command.Name() == "apply", nil, diagnostics.logger)
	if err != nil {
		return nil, fmt.Errorf("start service: %w", err)
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
	command.Flags().StringVar(&outputPath, "output-file", "-", "schema output path, or - for stdout")
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

func loadConfig(command *cobra.Command, paths []string, diagnostics *commandOutput) (*config.Config, error) {
	cfg, err := config.Load(paths...)
	if err != nil {
		return nil, fmt.Errorf("load configuration: %w", err)
	}
	if !diagnostics.explicitLevel {
		diagnostics.threshold.Set(cfg.ParsedLevel)
	}
	diagnostics.logger.DebugContext(command.Context(), "Loading application")
	return cfg, nil
}
