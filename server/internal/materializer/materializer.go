// Package materializer runs the SWE-bench materializer to generate Dockerfile
// build contexts, or validates a pre-generated directory.
package materializer

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/sebastian-l0/SWE-bench-cloudbuild/server/internal/manifest"
)

// Mode selects the materialization strategy.
type Mode string

const (
	// ModeCommand clones/updates SWE-bench and runs the materializer command.
	ModeCommand Mode = "command"
	// ModeGeneratedDir uses an existing output directory (tests/diagnostics).
	ModeGeneratedDir Mode = "generated-dir"
)

// Options configures a materialization run.
type Options struct {
	Mode        Mode
	OutputDir   string
	RepoDir     string // working copy of SWE-bench for command mode
	Dataset     string
	Split       string
	ImagePrefix string
	Tag         string
	Arch        string
	InstanceIDs []string
}

// Result captures materialization output and metadata for persistence.
type Result struct {
	OutputDir  string
	Command    string
	StdoutTail string
	StderrTail string
	StartedAt  time.Time
	FinishedAt time.Time
	Manifest   *manifest.Manifest
}

// Runner executes commands. The default uses os/exec; tests inject a fake.
type Runner interface {
	Run(ctx context.Context, dir, name string, args ...string) (stdout, stderr string, err error)
}

// ExecRunner runs commands via os/exec.
type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, dir, name string, args ...string) (string, string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err := cmd.Run()
	return out.String(), errOut.String(), err
}

const tailLimit = 4096

func tail(s string) string {
	if len(s) <= tailLimit {
		return s
	}
	return s[len(s)-tailLimit:]
}

// Materializer generates or validates Dockerfile contexts.
type Materializer struct {
	runner Runner
}

// New returns a Materializer using the given runner (nil uses ExecRunner).
func New(runner Runner) *Materializer {
	if runner == nil {
		runner = ExecRunner{}
	}
	return &Materializer{runner: runner}
}

// Materialize runs the configured mode and returns the validated manifest.
func (m *Materializer) Materialize(ctx context.Context, opts Options) (*Result, error) {
	res := &Result{OutputDir: opts.OutputDir, StartedAt: time.Now().UTC()}

	switch opts.Mode {
	case ModeGeneratedDir:
		if _, err := os.Stat(opts.OutputDir); err != nil {
			return nil, fmt.Errorf("materializer: output dir: %w", err)
		}
	case ModeCommand:
		args := commandArgs(opts)
		res.Command = "python -m swebench.harness.materialize_images " + strings.Join(args[3:], " ")
		stdout, stderr, err := m.runner.Run(ctx, opts.RepoDir, args[0], args[1:]...)
		res.StdoutTail = tail(stdout)
		res.StderrTail = tail(stderr)
		res.FinishedAt = time.Now().UTC()
		if err != nil {
			return res, fmt.Errorf("materializer: command failed: %w", err)
		}
	default:
		return nil, fmt.Errorf("materializer: unknown mode %q", opts.Mode)
	}

	parsed, err := manifest.ParseFile(opts.OutputDir)
	if err != nil {
		if res.FinishedAt.IsZero() {
			res.FinishedAt = time.Now().UTC()
		}
		return res, err
	}
	res.Manifest = parsed
	if res.FinishedAt.IsZero() {
		res.FinishedAt = time.Now().UTC()
	}
	return res, nil
}

// commandArgs builds the python materializer invocation: [python, -m, module, ...flags].
func commandArgs(opts Options) []string {
	args := []string{
		"python", "-m", "swebench.harness.materialize_images",
		"--image_prefix", opts.ImagePrefix,
		"--output_dir", opts.OutputDir,
	}
	if opts.Dataset != "" {
		args = append(args, "--dataset_name", opts.Dataset)
	}
	if opts.Split != "" {
		args = append(args, "--split", opts.Split)
	}
	if opts.Tag != "" {
		args = append(args, "--tag", opts.Tag)
	}
	if opts.Arch != "" {
		args = append(args, "--arch", opts.Arch)
	}
	if len(opts.InstanceIDs) > 0 {
		args = append(args, "--instance_ids")
		args = append(args, opts.InstanceIDs...)
	}
	return args
}
