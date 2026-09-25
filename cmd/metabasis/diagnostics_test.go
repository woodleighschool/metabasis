package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestProgressRequiresHumanTerminalOutput(t *testing.T) {
	terminal, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		t.Skip("requires a controlling terminal")
	}
	defer func() { _ = terminal.Close() }()
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("CI", "")
	for _, test := range []struct {
		name                                   string
		jsonOutput, stdoutPipe, stderrPipe, ci bool
		want                                   bool
	}{
		{name: "human terminal", want: true},
		{name: "JSON terminal", jsonOutput: true},
		{name: "stdout pipe", stdoutPipe: true, want: true},
		{name: "stderr pipe", stderrPipe: true},
		{name: "CI terminal", ci: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			command, output := newRootCommand()
			validate, _, err := command.Find([]string{"validate"})
			if err != nil {
				t.Fatal(err)
			}
			validate.SetOut(terminal)
			validate.SetErr(terminal)
			if test.stdoutPipe {
				validate.SetOut(io.Discard)
			}
			if test.stderrPipe {
				validate.SetErr(io.Discard)
			}
			if test.jsonOutput {
				if err := validate.Flags().Set("json", "true"); err != nil {
					t.Fatal(err)
				}
			}
			if test.ci {
				t.Setenv("CI", "true")
			}
			if err := output.start(validate); err != nil {
				t.Fatal(err)
			}
			if output.interactive != test.want {
				t.Fatalf("interactive = %t, want %t", output.interactive, test.want)
			}
		})
	}
}

func TestFiniteCommandReportsWithoutDiagnosticChatter(t *testing.T) {
	command, output := newRootCommand()
	var report, diagnostics bytes.Buffer
	command.SetOut(&report)
	command.SetErr(&diagnostics)
	command.AddCommand(&cobra.Command{Use: "probe", RunE: func(cmd *cobra.Command, _ []string) error {
		output.logger.Info("Resolving identity", "stage", true)
		output.logger.Info("Resolving identity", "stage_result", true)
		output.logger.Info("Applying memberships", "progress", true, "current", 1, "total", 1)
		output.logger.Debug("Request detail")
		_, err := fmt.Fprintln(cmd.OutOrStdout(), "Subject: user@example.invalid (unchanged)")
		return err
	}})
	command.SetArgs([]string{"probe"})
	executed, err := command.ExecuteC()
	output.finish(executed, err)
	if err != nil {
		t.Fatal(err)
	}
	if diagnostics.Len() != 0 || report.String() != "Subject: user@example.invalid (unchanged)\n" {
		t.Fatalf("report = %q, diagnostics = %q", report.String(), diagnostics.String())
	}
}

func TestFiniteCommandsPrintWarnings(t *testing.T) {
	command, output := newRootCommand()
	var report, diagnostics bytes.Buffer
	command.SetOut(&report)
	command.SetErr(&diagnostics)
	command.AddCommand(&cobra.Command{Use: "probe", RunE: func(_ *cobra.Command, _ []string) error {
		output.logger.Warn("Directory notice", "source", "fixture")
		return nil
	}})
	command.SetArgs([]string{"probe"})
	executed, err := command.ExecuteC()
	output.finish(executed, err)
	if err != nil || report.Len() != 0 || diagnostics.String() != "Warning: Directory notice; source=fixture\n" {
		t.Fatalf("error = %v, report = %q, diagnostics = %q", err, report.String(), diagnostics.String())
	}
}

func TestDaemonAlwaysUsesJSONWithStagesAtDebug(t *testing.T) {
	for _, level := range []string{"info", "debug"} {
		t.Run(level, func(t *testing.T) {
			command, output := newRootCommand()
			var diagnostics, report bytes.Buffer
			command.SetOut(&report)
			command.SetErr(&diagnostics)
			run, _, err := command.Find([]string{"run"})
			if err != nil {
				t.Fatal(err)
			}
			run.RunE = func(cmd *cobra.Command, _ []string) error {
				output.logger.InfoContext(cmd.Context(), "Resolving identity", "stage", true)
				output.logger.Info("Membership changed")
				return nil
			}
			command.SetArgs([]string{"run", "--log-level", level})
			executed, err := command.ExecuteC()
			output.finish(executed, err)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(diagnostics.String(), "Resolving identity") != (level == "debug") || report.Len() != 0 {
				t.Fatalf("daemon report = %q, diagnostics = %s", report.String(), diagnostics.String())
			}
			decoder := json.NewDecoder(&diagnostics)
			for {
				var record struct {
					Message string `json:"msg"`
					Level   string `json:"level"`
				}
				if err := decoder.Decode(&record); errors.Is(err, io.EOF) {
					break
				} else if err != nil {
					t.Fatal(err)
				}
				if record.Message == "Resolving identity" && record.Level != "DEBUG" {
					t.Fatalf("stage level = %s", record.Level)
				}
			}
		})
	}
}

func TestRemovedPresentationFlagsAreRejected(t *testing.T) {
	for _, commandName := range []string{"validate", "plan", "apply", "run"} {
		for _, flag := range []string{"--output=json", "--log-format=json", "--quiet", "--verbose", "--debug", "-q", "-v", "-d"} {
			command, output := newRootCommand()
			var report, diagnostics bytes.Buffer
			command.SetOut(&report)
			command.SetErr(&diagnostics)
			command.SetArgs([]string{commandName, flag})
			executed, err := command.ExecuteC()
			output.finish(executed, err)
			if err == nil || !strings.Contains(err.Error(), "unknown") || report.Len() != 0 {
				t.Fatalf("%s %s: error = %v, report = %q", commandName, flag, err, report.String())
			}
		}
	}
	for _, commandName := range []string{"validate", "plan", "apply"} {
		command, _ := newRootCommand()
		command.SetArgs([]string{commandName, "--log-level=debug"})
		if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "unknown flag") {
			t.Fatalf("%s accepts log-level: %v", commandName, err)
		}
	}
}

func TestStartupFailureLeavesStdoutEmpty(t *testing.T) {
	for _, jsonOutput := range []bool{false, true} {
		command, output := newRootCommand()
		var report, diagnostics bytes.Buffer
		command.SetOut(&report)
		command.SetErr(&diagnostics)
		args := []string{"plan", "--event", "event.json", "--config", "missing.yaml"}
		if jsonOutput {
			args = append(args, "--json")
		}
		command.SetArgs(args)
		executed, err := command.ExecuteC()
		output.finish(executed, err)
		if err == nil || report.Len() != 0 || strings.Count(diagnostics.String(), "Error:") != 1 {
			t.Fatalf("error = %v, report = %q, diagnostics = %q", err, report.String(), diagnostics.String())
		}
	}
}

func TestRunStartupFailureIsOneJSONDiagnostic(t *testing.T) {
	command, output := newRootCommand()
	var report, diagnostics bytes.Buffer
	command.SetOut(&report)
	command.SetErr(&diagnostics)
	command.SetArgs([]string{"run", "--config", "missing.yaml"})
	executed, err := command.ExecuteC()
	output.finish(executed, err)
	if err == nil || report.Len() != 0 {
		t.Fatalf("error = %v, report = %q", err, report.String())
	}
	var record struct {
		Level string `json:"level"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(diagnostics.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	if record.Level != "ERROR" || record.Error == "" {
		t.Fatalf("diagnostic = %#v", record)
	}
}

func TestRunHelpDoesNotLogServiceShutdown(t *testing.T) {
	for _, args := range [][]string{{"run", "--help"}, {"run", "-h"}, {"help", "run"}} {
		command, output := newRootCommand()
		var help, diagnostics bytes.Buffer
		command.SetOut(&help)
		command.SetErr(&diagnostics)
		command.SetArgs(args)
		executed, err := command.ExecuteC()
		output.finish(executed, err)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(help.String(), "metabasis run [flags]") || diagnostics.Len() != 0 {
			t.Fatalf("%v: help = %q, diagnostics = %q", args, help.String(), diagnostics.String())
		}
	}
}

func TestRunLogLevelOverridesConfiguration(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		command, output := newRootCommand()
		t.Setenv("METABASIS_LOG_LEVEL", "warn")
		path := writeCommandConfig(t, t.TempDir(), "config.yaml", commandConfig)
		var diagnostics bytes.Buffer
		command.SetOut(io.Discard)
		command.SetErr(&diagnostics)
		run, _, err := command.Find([]string{"run"})
		if err != nil {
			t.Fatal(err)
		}
		run.RunE = func(cmd *cobra.Command, _ []string) error {
			if _, err := (&cli{configPaths: []string{path}, output: output}).loadConfig(cmd); err != nil {
				return err
			}
			output.logger.Debug("Request detail")
			return nil
		}
		args := []string{"run"}
		if explicit {
			args = append(args, "--log-level", "debug")
		}
		command.SetArgs(args)
		executed, err := command.ExecuteC()
		output.finish(executed, err)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(diagnostics.String(), "Request detail") != explicit {
			t.Fatalf("configured level: %s", diagnostics.String())
		}
		if !explicit && diagnostics.Len() != 0 {
			t.Fatalf("warn level emitted routine diagnostics: %s", diagnostics.String())
		}
	}
}

func TestFiniteErrorsArePrintedOnceWithoutLogs(t *testing.T) {
	command, output := newRootCommand()
	var report, diagnostics bytes.Buffer
	command.SetOut(&report)
	command.SetErr(&diagnostics)
	command.AddCommand(&cobra.Command{Use: "probe", RunE: func(_ *cobra.Command, _ []string) error {
		output.logger.Info("Resolving identity", "stage", true)
		output.logger.Info("Resolving identity", "stage_result", true, "error", errors.New("directory unavailable"))
		return errors.New("directory unavailable\n\tHTTP 503")
	}})
	command.SetArgs([]string{"probe"})
	executed, err := command.ExecuteC()
	output.finish(executed, err)
	if err == nil || report.Len() != 0 || diagnostics.String() != "Error: directory unavailable HTTP 503\n" {
		t.Fatalf("error = %v, report = %q, diagnostics = %q", err, report.String(), diagnostics.String())
	}
}

func TestReportFailurePrintsConciseCauseOnStderr(t *testing.T) {
	command, output := newRootCommand()
	var report, diagnostics bytes.Buffer
	command.SetOut(&report)
	command.SetErr(&diagnostics)
	command.AddCommand(&cobra.Command{Use: "probe", RunE: func(cmd *cobra.Command, _ []string) error {
		failure := errors.New("directory unavailable")
		writeErr := writeJSON(cmd.OutOrStdout(), map[string]string{"error": failure.Error()})
		output.reportError = writeErr == nil
		return errors.Join(failure, writeErr)
	}})
	command.SetArgs([]string{"probe"})
	executed, err := command.ExecuteC()
	output.finish(executed, err)
	if err == nil || strings.Count(report.String(), "directory unavailable") != 1 {
		t.Fatalf("error = %v, report = %q, diagnostics = %q", err, report.String(), diagnostics.String())
	}
	if diagnostics.String() != "Error: directory unavailable\n" {
		t.Fatalf("diagnostics = %q", diagnostics.String())
	}
}

func TestCancellationIsNotAutomaticallyAnInterrupt(t *testing.T) {
	for _, interrupted := range []bool{false, true} {
		cmd, output := newRootCommand()
		var logs bytes.Buffer
		cmd.SetErr(&logs)
		ctx, cancel := context.WithCancelCause(t.Context())
		defer cancel(nil)
		cmd.SetContext(ctx)
		if interrupted {
			cancel(errInterrupted)
		}
		output.finish(cmd, context.Canceled)
		if strings.Contains(logs.String(), "interrupted") != interrupted {
			t.Fatalf("interrupted=%v: %s", interrupted, logs.String())
		}
	}
}
