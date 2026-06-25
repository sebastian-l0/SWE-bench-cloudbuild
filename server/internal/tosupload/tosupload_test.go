package tosupload

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/sebastian-l0/SWE-bench-cloudbuild/server/internal/config"
)

type fakeRunner struct {
	lookErr error
	runErr  error
	stdout  string
	stderr  string
	gotArgs [][]string
}

func (f *fakeRunner) Look(string) (string, error) {
	if f.lookErr != nil {
		return "", f.lookErr
	}
	return "/usr/local/bin/toscli", nil
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) (string, string, error) {
	f.gotArgs = append(f.gotArgs, append([]string{name}, args...))
	if len(args) > 0 && args[0] == "version" {
		return "toscli 1.2.3", "", nil
	}
	return f.stdout, f.stderr, f.runErr
}

func TestUploadSuccess(t *testing.T) {
	fr := &fakeRunner{stdout: "uploaded 3 files"}
	u := New(fr, "toscli")
	now := time.Date(2026, 6, 15, 10, 20, 30, 0, time.UTC)

	res, err := u.Upload(context.Background(), "/out", "my-bucket", "swe-cloudbuild", now)
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if res.URI != "tos://my-bucket/swe-cloudbuild/20260615102030/" {
		t.Fatalf("uri = %q", res.URI)
	}
	if res.Version != "toscli 1.2.3" {
		t.Fatalf("version = %q", res.Version)
	}
	if res.Summary != "uploaded 3 files" {
		t.Fatalf("summary = %q", res.Summary)
	}
}

func TestUploadEmptyParentPath(t *testing.T) {
	fr := &fakeRunner{}
	u := New(fr, "toscli")
	now := time.Date(2026, 6, 15, 1, 2, 3, 0, time.UTC)
	res, err := u.Upload(context.Background(), "/out", "b", "", now)
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if res.URI != "tos://b/20260615010203/" {
		t.Fatalf("uri = %q", res.URI)
	}
}

func TestUploadMissingBinary(t *testing.T) {
	fr := &fakeRunner{lookErr: errors.New("not found")}
	u := New(fr, "toscli")
	_, err := u.Upload(context.Background(), "/out", "b", "p", time.Now())
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("err = %v, want not found", err)
	}
}

func TestUploadMissingBucket(t *testing.T) {
	u := New(&fakeRunner{}, "toscli")
	if _, err := u.Upload(context.Background(), "/out", "", "p", time.Now()); err == nil {
		t.Fatal("expected bucket required error")
	}
}

func TestUploadFailure(t *testing.T) {
	fr := &fakeRunner{runErr: errors.New("exit 1"), stderr: "permission denied"}
	u := New(fr, "toscli")
	res, err := u.Upload(context.Background(), "/out", "b", "p", time.Now())
	if err == nil {
		t.Fatal("expected upload failure")
	}
	if res == nil || res.URI != "" {
		t.Fatalf("res = %#v, want no URI on failure", res)
	}
}

// TestUploadOutputRedaction confirms upload output can be redacted before being
// surfaced in logs/events.
func TestUploadOutputRedaction(t *testing.T) {
	leak := "AK" + "IAEXAMPLE1234567"
	fr := &fakeRunner{stdout: "uploading with VOLC_ACCESS" + "_KEY=" + leak}
	u := New(fr, "toscli")
	res, err := u.Upload(context.Background(), "/out", "b", "p", time.Now())
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	redacted := config.Redact(res.Summary)
	if strings.Contains(redacted, leak) {
		t.Fatalf("redacted summary leaked secret: %q", redacted)
	}
}
