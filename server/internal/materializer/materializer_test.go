package materializer

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeRunner struct {
	stdout string
	stderr string
	err    error
	gotDir string
	gotCmd []string
}

func (f *fakeRunner) Run(_ context.Context, dir, name string, args ...string) (string, string, error) {
	f.gotDir = dir
	f.gotCmd = append([]string{name}, args...)
	return f.stdout, f.stderr, f.err
}

func TestMaterializeGeneratedDir(t *testing.T) {
	m := New(nil)
	res, err := m.Materialize(context.Background(), Options{
		Mode:      ModeGeneratedDir,
		OutputDir: "../manifest/testdata/valid",
	})
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if res.Manifest == nil || len(res.Manifest.Images) != 3 {
		t.Fatalf("manifest = %#v", res.Manifest)
	}
}

func TestMaterializeGeneratedDirMissing(t *testing.T) {
	m := New(nil)
	_, err := m.Materialize(context.Background(), Options{
		Mode:      ModeGeneratedDir,
		OutputDir: "/nonexistent/path/here",
	})
	if err == nil {
		t.Fatal("expected error for missing dir")
	}
}

func TestMaterializeCommandSuccess(t *testing.T) {
	fr := &fakeRunner{stdout: "Materialized 1 base, 1 environment, and 1 instance image contexts"}
	m := New(fr)
	res, err := m.Materialize(context.Background(), Options{
		Mode:        ModeCommand,
		OutputDir:   "../manifest/testdata/valid",
		RepoDir:     "/tmp/swebench",
		Dataset:     "princeton-nlp/SWE-bench_Lite",
		Split:       "test",
		ImagePrefix: "ghcr.io/example/swebench-images",
		Tag:         "latest",
	})
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if fr.gotDir != "/tmp/swebench" {
		t.Fatalf("repo dir = %q", fr.gotDir)
	}
	joined := strings.Join(fr.gotCmd, " ")
	for _, want := range []string{"materialize_images", "--image_prefix", "ghcr.io/example/swebench-images", "--output_dir", "--dataset_name", "--split", "--tag"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("command missing %q: %s", want, joined)
		}
	}
	if res.Manifest == nil {
		t.Fatal("expected manifest parsed after command")
	}
	if !strings.Contains(res.StdoutTail, "Materialized") {
		t.Fatalf("stdout tail = %q", res.StdoutTail)
	}
}

func TestMaterializeCommandFailure(t *testing.T) {
	fr := &fakeRunner{stderr: "boom", err: errors.New("exit status 1")}
	m := New(fr)
	res, err := m.Materialize(context.Background(), Options{
		Mode:        ModeCommand,
		OutputDir:   "../manifest/testdata/valid",
		ImagePrefix: "x",
	})
	if err == nil {
		t.Fatal("expected command failure")
	}
	if res == nil || res.StderrTail != "boom" {
		t.Fatalf("stderr tail = %#v", res)
	}
	if res.Manifest != nil {
		t.Fatal("manifest must not be parsed on command failure")
	}
}

func TestMaterializeUnknownMode(t *testing.T) {
	m := New(nil)
	if _, err := m.Materialize(context.Background(), Options{Mode: "bogus"}); err == nil {
		t.Fatal("expected unknown mode error")
	}
}
