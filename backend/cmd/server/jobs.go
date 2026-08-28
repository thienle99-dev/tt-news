package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var errJobAlreadyRunning = errors.New("a job of this type is already running")

type jobRun struct {
	ID, TargetCount, CompletedCount, FailedCount int64
	Kind, Title, Trigger, Status, Stage, Detail  string
	StartedAt, FinishedAt                        string
}

func recoverInterruptedJobs(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `UPDATE job_runs SET status='failed',stage='Đã gián đoạn',detail='Server đã khởi động lại trước khi tác vụ hoàn tất',finished_at=? WHERE status='running'`, time.Now().UTC().Format(time.RFC3339))
	return err
}

func (s *server) startJob(ctx context.Context, kind, title, trigger string, target int) (jobRun, context.Context, error) {
	s.jobMu.Lock()
	defer s.jobMu.Unlock()
	var active jobRun
	err := s.db.QueryRowContext(ctx, `SELECT id,kind,title,trigger,status,stage,detail,target_count,completed_count,failed_count,started_at,finished_at FROM job_runs WHERE kind=? AND status='running' ORDER BY id DESC LIMIT 1`, kind).Scan(&active.ID, &active.Kind, &active.Title, &active.Trigger, &active.Status, &active.Stage, &active.Detail, &active.TargetCount, &active.CompletedCount, &active.FailedCount, &active.StartedAt, &active.FinishedAt)
	if err == nil {
		return active, nil, fmt.Errorf("%w: %d", errJobAlreadyRunning, active.ID)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return jobRun{}, nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := s.db.ExecContext(ctx, `INSERT INTO job_runs(kind,title,trigger,status,stage,target_count,started_at) VALUES(?,?,?,'running','Đang chuẩn bị',?,?)`, kind, title, trigger, target, now)
	if err != nil {
		return jobRun{}, nil, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return jobRun{}, nil, err
	}
	jobCtx, cancel := context.WithCancel(ctx)
	if s.jobCancels == nil {
		s.jobCancels = map[int64]context.CancelFunc{}
	}
	s.jobCancels[id] = cancel
	return jobRun{ID: id, Kind: kind, Title: title, Trigger: trigger, Status: "running", Stage: "Đang chuẩn bị", TargetCount: int64(target), StartedAt: now}, jobCtx, nil
}

func (s *server) updateJob(ctx context.Context, job jobRun, stage, detail string, completed, failed int64) {
	_, _ = s.db.ExecContext(ctx, `UPDATE job_runs SET stage=?,detail=?,completed_count=?,failed_count=? WHERE id=?`, stage, detail, completed, failed, job.ID)
}

func (s *server) finishJob(ctx context.Context, job jobRun, status, detail string, completed, failed int64) {
	_, _ = s.db.ExecContext(ctx, `UPDATE job_runs SET status=?,stage='Hoàn tất',detail=?,completed_count=?,failed_count=?,finished_at=? WHERE id=? AND status='running'`, status, detail, completed, failed, time.Now().UTC().Format(time.RFC3339), job.ID)
	s.jobMu.Lock()
	delete(s.jobCancels, job.ID)
	s.jobMu.Unlock()
	_, _ = s.db.ExecContext(ctx, `DELETE FROM job_runs WHERE status IN ('completed','failed','skipped') AND finished_at<>'' AND finished_at<?`, time.Now().UTC().AddDate(0, 0, -7).Format(time.RFC3339))
}

func (s *server) cancelJob(ctx context.Context, id int64) error {
	s.jobMu.Lock()
	cancel, active := s.jobCancels[id]
	s.jobMu.Unlock()
	if !active {
		return errors.New("job is not running")
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE job_runs SET status='skipped',stage='Đã hủy',detail='Đã hủy bởi quản trị viên',finished_at=? WHERE id=? AND status='running'`, time.Now().UTC().Format(time.RFC3339), id); err != nil {
		return err
	}
	cancel()
	return nil
}
