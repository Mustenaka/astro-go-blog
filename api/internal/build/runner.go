// Package build runs the release script: one build at a time, one more may queue, the rest are dropped.
package build

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/Mustenaka/astro-go-blog/api/internal/db"
)

// Status values stored in build_jobs.status.
const (
	StatusQueued    = "queued"
	StatusRunning   = "running"
	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"
)

// ErrBusy is returned when a build is running and another is already queued.
var ErrBusy = errors.New("a build is running and another is already queued")

// Job mirrors a build_jobs row.
type Job struct {
	ID         int64   `json:"id"`
	Status     string  `json:"status"`
	Trigger    string  `json:"trigger"`
	CreatedAt  string  `json:"createdAt"`
	StartedAt  *string `json:"startedAt"`
	FinishedAt *string `json:"finishedAt"`
	ExitCode   *int    `json:"exitCode"`
	LogPath    string  `json:"-"`
}

// Runner serialises builds.
type Runner struct {
	db     *db.DB
	script string
	logDir string
	// Env is added to the script environment (SITE_DIR, CONTENT_DIR, ...).
	Env []string

	mu      sync.Mutex
	running *Job
	queued  *Job
	wake    chan struct{}
	// OnFinish is called after each job (tests use it to synchronise).
	OnFinish func(*Job)
}

// New creates a runner writing logs under logDir. The loop starts with Start.
func New(d *db.DB, script, logDir string) *Runner {
	// The script runs with its own directory as cwd, so the path must be absolute.
	if script != "" {
		if abs, err := filepath.Abs(script); err == nil {
			script = abs
		}
	}
	return &Runner{db: d, script: script, logDir: logDir, wake: make(chan struct{}, 1)}
}

// Start launches the worker goroutine; it stops when ctx is cancelled.
func (r *Runner) Start(ctx context.Context) {
	// Anything left "running" from a previous process crashed with it.
	_, _ = r.db.ExecContext(ctx, `UPDATE build_jobs SET status = ?, finished_at = ? WHERE status IN (?, ?)`,
		StatusFailed, db.Now(), StatusRunning, StatusQueued)
	go r.loop(ctx)
}

// Trigger records a job. It returns the job, plus ErrBusy when one is running and one is queued.
func (r *Runner) Trigger(ctx context.Context, trigger string) (*Job, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.running != nil && r.queued != nil {
		return nil, ErrBusy
	}
	if r.script == "" {
		return nil, errors.New("RELEASE_SCRIPT is not configured")
	}
	res, err := r.db.ExecContext(ctx, `INSERT INTO build_jobs (status, trigger, created_at) VALUES (?, ?, ?)`, StatusQueued, trigger, db.Now())
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	job := &Job{ID: id, Status: StatusQueued, Trigger: trigger, CreatedAt: db.Now()}
	if r.running == nil {
		r.running = job
	} else {
		r.queued = job
	}
	select {
	case r.wake <- struct{}{}:
	default:
	}
	return job, nil
}

func (r *Runner) loop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-r.wake:
		}
		for {
			r.mu.Lock()
			job := r.running
			r.mu.Unlock()
			if job == nil {
				break
			}
			r.execute(ctx, job)
			r.mu.Lock()
			r.running = r.queued
			r.queued = nil
			r.mu.Unlock()
			if r.OnFinish != nil {
				r.OnFinish(job)
			}
		}
	}
}

func (r *Runner) execute(ctx context.Context, job *Job) {
	_ = os.MkdirAll(r.logDir, 0o755)
	logPath := filepath.Join(r.logDir, fmt.Sprintf("%d.log", job.ID))
	job.LogPath = logPath
	started := db.Now()
	job.Status = StatusRunning
	job.StartedAt = &started
	_, _ = r.db.ExecContext(ctx, `UPDATE build_jobs SET status = ?, started_at = ?, log_path = ? WHERE id = ?`, StatusRunning, started, logPath, job.ID)

	logFile, err := os.Create(logPath)
	if err != nil {
		r.finish(ctx, job, -1, fmt.Errorf("create log: %w", err))
		return
	}
	defer logFile.Close()
	fmt.Fprintf(logFile, "[build %d] trigger=%s script=%s started=%s\n", job.ID, job.Trigger, r.script, started)

	cmd := Command(ctx, r.script)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Env = append(os.Environ(), r.Env...)
	cmd.Dir = filepath.Dir(r.script)
	err = cmd.Run()
	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code = exitErr.ExitCode()
		} else {
			code = -1
		}
	}
	fmt.Fprintf(logFile, "[build %d] finished exit=%d\n", job.ID, code)
	r.finish(ctx, job, code, err)
}

func (r *Runner) finish(ctx context.Context, job *Job, code int, err error) {
	status := StatusSucceeded
	if err != nil {
		status = StatusFailed
		slog.Warn("build failed", "id", job.ID, "err", err)
	}
	finished := db.Now()
	job.Status, job.ExitCode, job.FinishedAt = status, &code, &finished
	_, _ = r.db.ExecContext(ctx, `UPDATE build_jobs SET status = ?, finished_at = ?, exit_code = ? WHERE id = ?`, status, finished, code, job.ID)
}

// Command builds the exec.Cmd for a release script: .ps1 via PowerShell, .sh via sh, else direct.
func Command(ctx context.Context, script string) *exec.Cmd {
	switch strings.ToLower(filepath.Ext(script)) {
	case ".ps1":
		shell := "powershell"
		if runtime.GOOS != "windows" {
			shell = "pwsh"
		}
		return exec.CommandContext(ctx, shell, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", script)
	case ".sh":
		return exec.CommandContext(ctx, "sh", script)
	case ".cmd", ".bat":
		return exec.CommandContext(ctx, "cmd", "/C", script)
	default:
		return exec.CommandContext(ctx, script)
	}
}

// Get returns one job by id.
func (r *Runner) Get(ctx context.Context, id int64) (*Job, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, status, trigger, created_at, started_at, finished_at, exit_code, COALESCE(log_path, '') FROM build_jobs WHERE id = ?`, id)
	return scanJob(row)
}

// Recent returns the newest n jobs.
func (r *Runner) Recent(ctx context.Context, n int) ([]*Job, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, status, trigger, created_at, started_at, finished_at, exit_code, COALESCE(log_path, '') FROM build_jobs ORDER BY id DESC LIMIT ?`, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// LogTail returns the last maxBytes of a job's log.
func (r *Runner) LogTail(job *Job, maxBytes int64) string {
	if job.LogPath == "" {
		return ""
	}
	f, err := os.Open(job.LogPath)
	if err != nil {
		return ""
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return ""
	}
	start := info.Size() - maxBytes
	if start < 0 {
		start = 0
	}
	buf := make([]byte, info.Size()-start)
	n, _ := f.ReadAt(buf, start)
	return string(buf[:n])
}

type scanner interface{ Scan(dest ...any) error }

func scanJob(s scanner) (*Job, error) {
	var j Job
	var started, finished sql.NullString
	var code sql.NullInt64
	if err := s.Scan(&j.ID, &j.Status, &j.Trigger, &j.CreatedAt, &started, &finished, &code, &j.LogPath); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}
	if started.Valid {
		j.StartedAt = &started.String
	}
	if finished.Valid {
		j.FinishedAt = &finished.String
	}
	if code.Valid {
		c := int(code.Int64)
		j.ExitCode = &c
	}
	return &j, nil
}

// Wait blocks until no job is running or queued, or the timeout passes. Test helper.
func (r *Runner) Wait(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		r.mu.Lock()
		idle := r.running == nil && r.queued == nil
		r.mu.Unlock()
		if idle {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}
