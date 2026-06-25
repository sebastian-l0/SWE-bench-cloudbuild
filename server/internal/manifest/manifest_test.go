package manifest

import (
	"strings"
	"testing"
)

func TestParseFileValidThreeLayer(t *testing.T) {
	m, err := ParseFile("testdata/valid")
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(m.Images) != 3 {
		t.Fatalf("images = %d, want 3", len(m.Images))
	}
	if m.Images[0].Layer != LayerBase || m.Images[1].Layer != LayerEnv || m.Images[2].Layer != LayerInstance {
		t.Fatalf("layer order = %s,%s,%s", m.Images[0].Layer, m.Images[1].Layer, m.Images[2].Layer)
	}
	if m.Images[1].DependsOnKey != "sweb.base.py.x86_64" {
		t.Fatalf("env depends on = %q", m.Images[1].DependsOnKey)
	}
	if m.Images[2].DependsOnKey != "sweb.env.py.x86_64.abc123" {
		t.Fatalf("instance depends on = %q", m.Images[2].DependsOnKey)
	}
	if m.Images[2].InstanceID != "django__django-11099" {
		t.Fatalf("instance id = %q", m.Images[2].InstanceID)
	}
	if !strings.Contains(m.Images[0].RawJSON, "sweb.base.py.x86_64") {
		t.Fatalf("raw json missing: %s", m.Images[0].RawJSON)
	}
}

func TestParseRejectsMissingDependencyKey(t *testing.T) {
	data := []byte(`{
		"base_images":[{"local_image_key":"b1","target_image":"t","context_path":"contexts/base/b1","dockerfile":"Dockerfile"}],
		"env_images":[{"local_image_key":"e1","target_image":"t","context_path":"contexts/env/e1","dockerfile":"Dockerfile","base_local_image_key":"missing"}]
	}`)
	_, err := Parse(data, "")
	if err == nil || !strings.Contains(err.Error(), "missing key") {
		t.Fatalf("err = %v, want missing key", err)
	}
}

func TestParseRejectsDuplicateKey(t *testing.T) {
	data := []byte(`{
		"base_images":[
			{"local_image_key":"dup","target_image":"t","context_path":"contexts/base/a","dockerfile":"Dockerfile"},
			{"local_image_key":"dup","target_image":"t","context_path":"contexts/base/b","dockerfile":"Dockerfile"}
		]
	}`)
	_, err := Parse(data, "")
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("err = %v, want duplicate", err)
	}
}

func TestParseRejectsPathEscape(t *testing.T) {
	data := []byte(`{
		"base_images":[{"local_image_key":"b1","target_image":"t","context_path":"../../etc","dockerfile":"Dockerfile"}]
	}`)
	_, err := Parse(data, "/tmp/out")
	if err == nil || !strings.Contains(err.Error(), "escape") {
		t.Fatalf("err = %v, want escape", err)
	}
}

func TestParseRejectsAbsolutePath(t *testing.T) {
	data := []byte(`{
		"base_images":[{"local_image_key":"b1","target_image":"t","context_path":"/etc/passwd","dockerfile":"Dockerfile"}]
	}`)
	_, err := Parse(data, "/tmp/out")
	if err == nil || !strings.Contains(err.Error(), "absolute") {
		t.Fatalf("err = %v, want absolute", err)
	}
}

func TestParseRejectsEmptyBaseImages(t *testing.T) {
	_, err := Parse([]byte(`{"env_images":[]}`), "")
	if err == nil || !strings.Contains(err.Error(), "no base_images") {
		t.Fatalf("err = %v", err)
	}
}

func TestParseDefaultsDockerfile(t *testing.T) {
	data := []byte(`{
		"base_images":[{"local_image_key":"b1","target_image":"t","context_path":"contexts/base/b1"}]
	}`)
	m, err := Parse(data, "")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if m.Images[0].Dockerfile != "Dockerfile" {
		t.Fatalf("dockerfile default = %q", m.Images[0].Dockerfile)
	}
}
