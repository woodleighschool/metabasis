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

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/woodleighschool/metabasis/internal/app"
	"github.com/woodleighschool/metabasis/internal/config"
	"github.com/woodleighschool/metabasis/internal/httpapi"
	"github.com/woodleighschool/metabasis/internal/intent"
	"github.com/woodleighschool/metabasis/internal/metrics"
	"github.com/woodleighschool/metabasis/internal/reconcile"
)

type cli struct {
	configPaths []string
	output      *commandOutput
}

func newRootCommand() (*cobra.Command, *commandOutput) {
	c := &cli{output: &commandOutput{}}
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
	output := c.output
	command.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		return output.start(cmd)
	}
	command.SetVersionTemplate(fmt.Sprintf("metabasis %s (commit %s, built %s)\n", version, commit, date))
	command.PersistentFlags().Bool("no-progress", false, "Disable terminal progress")
	command.PersistentFlags().StringArrayVar(
		&c.configPaths,
		"config",
		defaultConfigPaths(),
		"path to a YAML configuration file; may be repeated in overlay order",
	)
	command.AddCommand(
		c.validateCommand(),
		c.planCommand(),
		c.runCommand(),
		c.intentsCommand(),
		c.applyCommand(),
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

func (c *cli) validateCommand() *cobra.Command {
	var jsonOutput bool
	command := &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration and identity expressions",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if _, err := c.loadConfig(command); err != nil {
				return fmt.Errorf("validate configuration: %w", err)
			}
			if jsonOutput {
				return writeJSON(command.OutOrStdout(), map[string]bool{"valid": true})
			}
			_, err := fmt.Fprintln(command.OutOrStdout(), newTextStyle(command.OutOrStdout()).paint("✓ Configuration is valid.", color.FgHiGreen))
			return err
		},
	}
	command.Flags().BoolVar(&jsonOutput, "json", false, "Write one JSON report")
	return command
}

func (c *cli) planCommand() *cobra.Command {
	var eventPath string
	var jsonOutput bool
	command := &cobra.Command{
		Use:   "plan",
		Short: "Print the read-only identity plan for a canonical event",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if eventPath == "" {
				return fmt.Errorf("--event is required")
			}
			cfg, err := c.loadConfig(command)
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
			application, err := app.Build(command.Context(), cfg, false, nil, c.output.logger)
			if err != nil {
				return fmt.Errorf("start read-only service: %w", err)
			}
			plan, planErr := application.Reconciler.PlanEvent(command.Context(), event)
			c.output.stop()
			writeErr := writePlan(command.OutOrStdout(), jsonOutput, plan, planErr)
			c.output.reportError = planErr != nil && writeErr == nil
			application.Close()
			return errors.Join(planErr, writeErr)
		},
	}
	command.Flags().StringVar(&eventPath, "event", "", "path to a canonical intent JSON file")
	command.Flags().BoolVar(&jsonOutput, "json", false, "Write one JSON report")
	return command
}

func (c *cli) runCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "run",
		Short: "Serve webhooks and reconcile scheduled identity state",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			cfg, err := c.loadConfig(command)
			if err != nil {
				return err
			}
			logger := c.output.logger
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
	command.Flags().StringVar(&c.output.level, "log-level", "info", "Log level: debug, info, warn or error")
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

func (c *cli) intentsCommand() *cobra.Command {
	command := &cobra.Command{Use: "intents", Short: "Inspect accepted intents", Args: cobra.NoArgs}
	command.AddCommand(c.intentsListCommand(), c.intentsShowCommand())
	return command
}

func (c *cli) intentsListCommand() *cobra.Command {
	var jsonOutput bool
	command := &cobra.Command{
		Use:   "list",
		Short: "List accepted intents",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			application, err := c.buildOperationalApp(command)
			if err != nil {
				return err
			}
			defer application.Close()
			intents, err := application.Store.ListAllIntents(command.Context())
			if err != nil {
				return err
			}
			return writeIntents(command.OutOrStdout(), jsonOutput, intents, time.Now().UTC())
		},
	}
	command.Flags().BoolVar(&jsonOutput, "json", false, "Write one JSON report")
	return command
}

func (c *cli) intentsShowCommand() *cobra.Command {
	var jsonOutput bool
	command := &cobra.Command{
		Use:   "show <source> <id>",
		Short: "Show one accepted intent and its subject reconciliation state",
		Args:  cobra.ExactArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			application, err := c.buildOperationalApp(command)
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
			return writeIntent(command.OutOrStdout(), jsonOutput, accepted, state, time.Now().UTC())
		},
	}
	command.Flags().BoolVar(&jsonOutput, "json", false, "Write one JSON report")
	return command
}

func (c *cli) applyCommand() *cobra.Command {
	var subject string
	var all bool
	var includeUnchanged bool
	var jsonOutput bool
	command := &cobra.Command{
		Use:   "apply",
		Short: "Reconcile one subject or all accepted subjects now",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if (strings.TrimSpace(subject) == "") == !all {
				return fmt.Errorf("set exactly one of --subject or --all")
			}
			application, err := c.buildOperationalApp(command)
			if err != nil {
				return err
			}
			defer application.Close()
			// Human reports stream each subject as it finishes; JSON is one
			// document at the end.
			finished := func(result reconcile.Result) error {
				if jsonOutput {
					return nil
				}
				return c.output.subjectDone(command.OutOrStdout(), result, includeUnchanged || !all)
			}
			var results []reconcile.Result
			if all {
				application.Reconciler.SubjectDone = finished
				results, err = application.Reconciler.ReconcileAll(command.Context())
			} else {
				var result reconcile.Result
				result, err = application.Reconciler.ReconcileSubject(command.Context(), strings.TrimSpace(subject))
				results = []reconcile.Result{result}
				err = errors.Join(err, finished(result))
			}
			c.output.stop()
			writeErr := writeApplyReport(command.OutOrStdout(), jsonOutput, includeUnchanged || !all, results, err)
			c.output.reportError = err != nil && writeErr == nil
			return errors.Join(err, writeErr)
		},
	}
	command.Flags().StringVar(&subject, "subject", "", "subject to reconcile")
	command.Flags().BoolVar(&all, "all", false, "Reconcile all accepted subjects")
	command.Flags().BoolVar(&includeUnchanged, "include-unchanged", false, "Include unchanged subjects in the human report")
	command.Flags().BoolVar(&jsonOutput, "json", false, "Write one JSON report")
	return command
}

func (c *cli) buildOperationalApp(command *cobra.Command) (*app.App, error) {
	cfg, err := c.loadConfig(command)
	if err != nil {
		return nil, err
	}
	application, err := app.Build(command.Context(), cfg, command.Name() == "apply", nil, c.output.logger)
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
			_, err := fmt.Fprintf(command.OutOrStdout(), "metabasis %s (commit %s, built %s)\n", version, commit, date)
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

func (c *cli) loadConfig(command *cobra.Command) (*config.Config, error) {
	cfg, err := config.Load(c.configPaths...)
	if err != nil {
		return nil, fmt.Errorf("load configuration: %w", err)
	}
	if c.output.daemon && !c.output.explicitLevel {
		c.output.threshold.Set(cfg.ParsedLevel)
	}
	c.output.logger.DebugContext(command.Context(), "Loading application")
	return cfg, nil
}
