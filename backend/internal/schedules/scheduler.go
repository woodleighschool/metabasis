package schedules

import (
	"context"
	"log/slog"
	"time"

	"github.com/robfig/cron/v3"
)

type Job func(context.Context) error

type Scheduler struct {
	cron   *cron.Cron
	logger *slog.Logger
}

func NewScheduler(logger *slog.Logger) *Scheduler {
	return &Scheduler{
		cron:   cron.New(),
		logger: logger,
	}
}

func (s *Scheduler) Add(spec, name string, job Job) error {
	_, err := s.cron.AddFunc(spec, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		if err := job(ctx); err != nil {
			s.logger.Error("job failed", "job", name, "err", err)
		} else {
			s.logger.Debug("job completed successfully", "job", name, "err", err)
		}
	})

	return err
}

func (s *Scheduler) Run() {
	s.cron.Run()
}

func (s *Scheduler) Start() {
	s.cron.Start()
}

func (s *Scheduler) Stop() context.Context {
	return s.cron.Stop()
}
