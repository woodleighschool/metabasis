package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/woodleighschool/metabasis/internal/planner"
	"github.com/woodleighschool/metabasis/internal/reconcile"
)

func TestRunLoopCancelsInFlightReconciliation(t *testing.T) {
	t.Parallel()
	started := make(chan struct{})
	service := blockingReconciler{started: started}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, &slog.HandlerOptions{Level: slog.LevelInfo}))
	go func() {
		runLoop(ctx, time.Hour, service, nil, logger)
		close(done)
	}()
	<-started
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("runLoop did not stop after cancellation")
	}
	if strings.Contains(output.String(), "reconciliation cycle failed") {
		t.Errorf("shutdown logged a reconciliation failure: %s", output.String())
	}
}

func TestRunCycleKeepsRoutineSuccessAtDebug(t *testing.T) {
	reconcileSubjects := func(context.Context) ([]reconcile.Result, error) {
		return []reconcile.Result{{Subject: "user@example.invalid", Plan: planner.Plan{Rule: "staff"}}}, nil
	}

	var infoOutput bytes.Buffer
	runCycle(t.Context(), reconcileSubjects, slog.New(slog.NewJSONHandler(&infoOutput, &slog.HandlerOptions{Level: slog.LevelInfo})))
	if records := decodeLoopLogRecords(t, infoOutput.Bytes()); len(records) != 0 {
		t.Errorf("info logs contain routine success: %#v", records)
	}

	var debugOutput bytes.Buffer
	runCycle(t.Context(), reconcileSubjects, slog.New(slog.NewJSONHandler(&debugOutput, &slog.HandlerOptions{Level: slog.LevelDebug})))
	records := decodeLoopLogRecords(t, debugOutput.Bytes())
	want := []loopLogRecord{
		{Level: "DEBUG", Message: "subject reconciled"},
		{Level: "DEBUG", Message: "reconciliation cycle complete"},
	}
	if len(records) != len(want) {
		t.Fatalf("debug log records = %#v, want %#v", records, want)
	}
	for index := range want {
		if records[index] != want[index] {
			t.Errorf("debug log record %d = %#v, want %#v", index, records[index], want[index])
		}
	}
}

func TestRunCycleLogsMembershipChangesAtInfo(t *testing.T) {
	reconcileSubjects := func(context.Context) ([]reconcile.Result, error) {
		return []reconcile.Result{{
			Subject: "user@example.invalid",
			Plan:    planner.Plan{Rule: "staff", AddGroups: []string{"allow"}},
		}}, nil
	}
	var output bytes.Buffer
	runCycle(t.Context(), reconcileSubjects, slog.New(slog.NewJSONHandler(&output, &slog.HandlerOptions{Level: slog.LevelInfo})))

	records := decodeLoopLogRecords(t, output.Bytes())
	want := []loopLogRecord{{Level: "INFO", Message: "subject reconciled"}}
	if len(records) != len(want) || records[0] != want[0] {
		t.Errorf("info log records = %#v, want %#v", records, want)
	}
}

func TestRunCycleDoesNotLogParentCancellationAsFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	reconcileSubjects := func(context.Context) ([]reconcile.Result, error) {
		return []reconcile.Result{{Subject: "user@example.invalid"}}, context.Canceled
	}
	var output bytes.Buffer
	runCycle(ctx, reconcileSubjects, slog.New(slog.NewJSONHandler(&output, &slog.HandlerOptions{Level: slog.LevelDebug})))

	if records := decodeLoopLogRecords(t, output.Bytes()); len(records) != 0 {
		t.Errorf("shutdown produced log records: %#v", records)
	}
}

type loopLogRecord struct {
	Level   string `json:"level"`
	Message string `json:"msg"`
}

func decodeLoopLogRecords(t *testing.T, data []byte) []loopLogRecord {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(data))
	var records []loopLogRecord
	for {
		var record loopLogRecord
		if err := decoder.Decode(&record); err != nil {
			if errors.Is(err, io.EOF) {
				return records
			}
			t.Fatalf("decode log: %v", err)
		}
		records = append(records, record)
	}
}

type blockingReconciler struct {
	started chan<- struct{}
}

func (r blockingReconciler) ReconcileAll(ctx context.Context) ([]reconcile.Result, error) {
	close(r.started)
	<-ctx.Done()
	return nil, ctx.Err()
}

func (blockingReconciler) ReconcileDue(context.Context) ([]reconcile.Result, error) {
	return nil, nil
}

func (blockingReconciler) NextWake(context.Context) (*time.Time, error) {
	return nil, nil
}
