// Package crawler provides the shared scheduler contract for non-RSS crawlers.
package crawler

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/robfig/cron/v3"
)

// Job is implemented by each crawler that needs a cron schedule.
type Job interface {
	Name() string
	Schedule() string
	Run(context.Context)
}

// Start validates and starts all jobs. Each job runs immediately once, then on
// its five-field cron schedule in UTC. It stops gracefully with ctx.
func Start(ctx context.Context, jobs ...Job) error {
	runner := cron.New(cron.WithLocation(time.UTC))
	for _, job := range jobs {
		job := job
		if _, err := runner.AddFunc(job.Schedule(), func() { job.Run(ctx) }); err != nil {
			return fmt.Errorf("%s: invalid cron schedule: %w", job.Name(), err)
		}
		go job.Run(ctx)
	}
	runner.Start()

	go func() {
		<-ctx.Done()
		stop := runner.Stop()
		<-stop.Done()
		log.Print("crawler scheduler stopped")
	}()
	return nil
}
