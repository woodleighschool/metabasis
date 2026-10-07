package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/woodleighschool/metabasis/internal/reconcile"
)

// commandOutput owns stderr. Finite commands show a live region of each
// subject's stages in a terminal and print warnings; run writes JSON logs.
type commandOutput struct {
	activityEnabled bool
	mu              sync.Mutex
	out             io.Writer
	style           textStyle
	logger          *slog.Logger
	progress        *terminalProgress
	interactive     bool
	reportError     bool
	level           string
	threshold       slog.LevelVar
	explicitLevel   bool
	daemon          bool
}

func (o *commandOutput) start(cmd *cobra.Command) error {
	o.out = cmd.ErrOrStderr()
	o.style = newTextStyle(o.out)
	o.daemon = cmd.Name() == "run"
	o.explicitLevel = o.daemon && cmd.Flags().Changed("log-level")
	o.threshold.Set(slog.LevelInfo)
	if o.daemon {
		switch strings.ToLower(o.level) {
		case "debug", "info", "warn", "error":
		default:
			return fmt.Errorf("invalid log level %q: use debug, info, warn or error", o.level)
		}
		if err := o.threshold.UnmarshalText([]byte(o.level)); err != nil {
			return fmt.Errorf("invalid log level %q: use debug, info, warn or error", o.level)
		}
	}
	jsonOutput, _ := cmd.Flags().GetBool("json")
	noProgress, _ := cmd.Flags().GetBool("no-progress")
	o.activityEnabled = !o.daemon && !jsonOutput && !noProgress && cmd.Name() != "schema"
	o.interactive = o.activityEnabled && terminalOutput(o.out) && os.Getenv("CI") == ""
	o.logger = slog.New(&activityHandler{Handler: slog.NewJSONHandler(o.out, &slog.HandlerOptions{Level: &o.threshold}), output: o})
	cmd.SetOut(reportWriter{Writer: cmd.OutOrStdout(), output: o})
	return nil
}

func (o *commandOutput) finish(cmd *cobra.Command, err error) {
	interrupted := cmd.Context() != nil && errors.Is(context.Cause(cmd.Context()), errInterrupted)
	terminated := cmd.Context() != nil && errors.Is(context.Cause(cmd.Context()), errTerminated)
	if interrupted {
		err = errInterrupted
	}
	o.stop()
	if err == nil && o.logger == nil {
		return
	}
	if cmd.Name() == "run" {
		if o.logger == nil {
			o.logger = slog.New(slog.NewJSONHandler(cmd.ErrOrStderr(), nil))
		}
		switch {
		case interrupted || terminated:
			o.logger.Info("Service stopped")
		case err != nil:
			o.logger.Error("Service failed", "error", err)
		default:
			o.logger.Info("Service stopped")
		}
		return
	}
	if err == nil {
		return
	}
	message := err.Error()
	if o.reportError {
		message = strings.SplitN(message, "\n", 2)[0]
	}
	style := newTextStyle(cmd.ErrOrStderr())
	label := style.paint("✗", color.FgHiRed)
	if interrupted {
		message, label = "Interrupted.", style.paint("–", color.FgHiYellow)
	}
	_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "%s %s\n", label, reportText(strings.Join(strings.Fields(message), " ")))
}

func (o *commandOutput) stop() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.interactive = false
	o.activityEnabled = false
	if o.progress != nil {
		o.progress.stop()
		o.progress = nil
	}
}

// reportWriter clears the live region before the final report is written.
type reportWriter struct {
	io.Writer
	output *commandOutput
}

func (w reportWriter) Write(data []byte) (int, error) {
	w.output.stop()
	return w.Writer.Write(data)
}

// subjectDone streams a finished subject's report block to out. In a
// terminal, a subject the report leaves out still leaves its outcome line in
// place of its live activity.
func (o *commandOutput) subjectDone(out io.Writer, result reconcile.Result, includeUnchanged bool) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.progress != nil {
		o.progress.complete(result.Subject)
	}
	// Streaming goes around the final report's writer, which ends progress.
	if writer, ok := out.(reportWriter); ok {
		out = writer.Writer
	}
	style := newTextStyle(out)
	var text string
	switch {
	case includeUnchanged || subjectStatus(result) != "unchanged":
		text = renderSubject(style, result)
	default:
		return nil
	}
	if o.progress != nil {
		return o.progress.write(out, text)
	}
	_, err := io.WriteString(out, text)
	return err
}

// activityHandler sends stage records to the live region and prints warnings.
type activityHandler struct {
	slog.Handler
	output *commandOutput
	attrs  []slog.Attr
}

func (h *activityHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &activityHandler{Handler: h.Handler.WithAttrs(attrs), output: h.output, attrs: append(append([]slog.Attr(nil), h.attrs...), attrs...)}
}

func (h *activityHandler) WithGroup(name string) slog.Handler {
	return &activityHandler{Handler: h.Handler.WithGroup(name), output: h.output, attrs: h.attrs}
}

func (h *activityHandler) Handle(ctx context.Context, record slog.Record) error {
	a := readActivity(record, h.attrs)
	o := h.output
	if o.daemon {
		if a.stage || a.progress || a.status || subjectResult(record) {
			record.Level = slog.LevelDebug
		}
		if !h.Enabled(ctx, record.Level) {
			return nil
		}
		return h.Handler.Handle(ctx, record)
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if a.stage || a.progress || a.status {
		if o.activityEnabled || o.interactive {
			if o.progress == nil {
				o.progress = newTerminalProgress(o.out)
				if !o.interactive {
					o.progress.startPlain()
				}
			}
			o.progress.update(a)
		}
		return nil
	}
	if record.Level < slog.LevelWarn {
		return nil
	}
	var attrs []string
	read := func(attr slog.Attr) bool {
		attrs = append(attrs, attr.Key+"="+attr.Value.String())
		return true
	}
	for _, attr := range h.attrs {
		read(attr)
	}
	record.Attrs(read)
	message := record.Message
	if len(attrs) > 0 {
		message += "; " + strings.Join(attrs, "; ")
	}
	line := o.style.paint("!", color.FgHiYellow) + " " + reportText(message) + "\n"
	if o.progress != nil {
		return o.progress.write(o.out, line)
	}
	_, err := io.WriteString(o.out, line)
	return err
}

// subjectResult reports whether a record ends a subject's reconciliation.
func subjectResult(record slog.Record) bool {
	found := false
	record.Attrs(func(attr slog.Attr) bool {
		found = attr.Key == "subject_result" && attr.Value.Kind() == slog.KindBool && attr.Value.Bool()
		return !found
	})
	return found
}
