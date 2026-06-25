// Package tosupload wraps the toscli binary to upload a generated directory to a
// TOS timestamp prefix.
package tosupload

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Runner abstracts command execution for testability.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) (stdout, stderr string, err error)
	Look(name string) (string, error)
}

// ExecRunner runs commands via os/exec.
type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args ...string) (string, string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err := cmd.Run()
	return out.String(), errOut.String(), err
}

func (ExecRunner) Look(name string) (string, error) {
	return exec.LookPath(name)
}

// Result describes a completed upload.
type Result struct {
	URI        string
	Prefix     string
	Version    string
	Summary    string
	StartedAt  time.Time
	FinishedAt time.Time
}

// Uploader uploads directories to TOS via toscli.
type Uploader struct {
	runner Runner
	binary string
}

// New returns an Uploader. runner nil uses ExecRunner; binary empty uses "toscli".
func New(runner Runner, binary string) *Uploader {
	if runner == nil {
		runner = ExecRunner{}
	}
	if binary == "" {
		binary = "toscli"
	}
	return &Uploader{runner: runner, binary: binary}
}

// timestamp formats now as the yyyyMMddHHmmss prefix segment.
func timestamp(now time.Time) string {
	return now.UTC().Format("20060102150405")
}

// Upload uploads localDir to tos://{bucket}/{parentPath}/{timestamp}/.
// It first verifies toscli is available and captures its version.
func (u *Uploader) Upload(ctx context.Context, localDir, bucket, parentPath string, now time.Time) (*Result, error) {
	if bucket == "" {
		return nil, fmt.Errorf("tosupload: bucket is required")
	}
	if _, err := u.runner.Look(u.binary); err != nil {
		return nil, fmt.Errorf("tosupload: %s not found: %w", u.binary, err)
	}

	res := &Result{StartedAt: time.Now().UTC()}
	if ver, _, err := u.runner.Run(ctx, u.binary, "version"); err == nil {
		res.Version = strings.TrimSpace(ver)
	}

	prefix := joinPrefix(parentPath, timestamp(now))
	dst := fmt.Sprintf("tos://%s/%s/", bucket, prefix)

	stdout, stderr, err := u.runner.Run(ctx, u.binary, "cp", "-r", localDir, dst)
	res.FinishedAt = time.Now().UTC()
	if err != nil {
		return res, fmt.Errorf("tosupload: upload failed: %w: %s", err, strings.TrimSpace(stderr))
	}
	res.URI = dst
	res.Prefix = prefix
	res.Summary = strings.TrimSpace(stdout)
	return res, nil
}

func joinPrefix(parent, ts string) string {
	parent = strings.Trim(parent, "/")
	if parent == "" {
		return ts
	}
	return parent + "/" + ts
}
