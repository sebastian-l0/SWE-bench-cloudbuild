package config

import "testing"

func TestLoadDefaultsAndEnvironmentOverrides(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://custom:custom@localhost:5433/custom?sslmode=disable")
	t.Setenv("SWE_CLOUDBUILD_VOLC_TARGET", "pre")
	t.Setenv("SWE_CLOUDBUILD_TOS_BUCKET", "demo-bucket")
	t.Setenv("SWE_CLOUDBUILD_TOS_PREFIX", "demo/prefix")
	t.Setenv("SWE_CLOUDBUILD_REGISTRY_NAMESPACE", "registry.example.com/swe")
	t.Setenv("VOLC_ACCESS_KEY", "AKIA"+"loadtest")
	t.Setenv("VOLC_SECRET_KEY", "secret"+"loadtest")
	t.Setenv("SWE_CLOUDBUILD_MOCK", "1")

	cfg := Load()

	if cfg.DatabaseURL != "postgres://custom:custom@localhost:5433/custom?sslmode=disable" {
		t.Fatalf("DatabaseURL = %q", cfg.DatabaseURL)
	}
	if cfg.VolcTarget != "pre" {
		t.Fatalf("VolcTarget = %q", cfg.VolcTarget)
	}
	if cfg.VolcAccessKey != "AKIA"+"loadtest" || cfg.VolcSecretKey != "secret"+"loadtest" {
		t.Fatalf("Volc credentials = %q / %q", cfg.VolcAccessKey, cfg.VolcSecretKey)
	}
	if cfg.TOS.Bucket != "demo-bucket" || cfg.TOS.ParentPath != "demo/prefix" {
		t.Fatalf("TOS = %#v", cfg.TOS)
	}
	if cfg.RegistryNamespace != "registry.example.com/swe" {
		t.Fatalf("RegistryNamespace = %q", cfg.RegistryNamespace)
	}
	if !cfg.MockMode {
		t.Fatal("MockMode = false, want true")
	}
	if cfg.Concurrency.Base != 1 || cfg.Concurrency.Env != 10 || cfg.Concurrency.Instance != 20 {
		t.Fatalf("Concurrency = %#v", cfg.Concurrency)
	}
	if cfg.Materializer.RepoURL != "https://github.com/sebastian-l0/SWE-bench" {
		t.Fatalf("Materializer.RepoURL = %q", cfg.Materializer.RepoURL)
	}
	if cfg.Materializer.Ref != "feature/materialize-image-contexts" {
		t.Fatalf("Materializer.Ref = %q", cfg.Materializer.Ref)
	}
}

func TestRedactMasksSensitiveKeysAndValues(t *testing.T) {
	accessKey := "AKIA" + "1234567890"
	secretKey := "super" + "secret"
	databaseURL := "postgres://" + "user:pass@host/db"
	token := "token" + "-value"
	input := "VOLC_ACCESS" + "_KEY=" + accessKey + " VOLC_SECRET" + "_KEY=" + secretKey + " DATABASE" + "_URL=" + databaseURL + " Authorization: Bearer " + token
	got := Redact(input)

	for _, leaked := range []string{accessKey, secretKey, databaseURL, token} {
		if contains(got, leaked) {
			t.Fatalf("redacted output leaked %q in %q", leaked, got)
		}
	}
	if !contains(got, "<redacted>") {
		t.Fatalf("redacted output = %q, want marker", got)
	}
}

func TestRedactMasksJSONSecretValues(t *testing.T) {
	accessKey := "AKIA" + "jsonsecret"
	dbURL := "postgres://" + "u:p@h/db"
	input := `{"volcAccessKey":"` + accessKey + `","databaseUrl": "` + dbURL + `","registryNamespace":"keep-me"}`
	got := Redact(input)

	for _, leaked := range []string{accessKey, dbURL} {
		if contains(got, leaked) {
			t.Fatalf("redacted JSON leaked %q in %q", leaked, got)
		}
	}
	if !contains(got, "keep-me") {
		t.Fatalf("redacted JSON dropped non-secret value: %q", got)
	}
}

func TestRedactDoesNotTouchUnrelatedKeys(t *testing.T) {
	input := "task=build worktree=clean"
	if got := Redact(input); got != input {
		t.Fatalf("redact altered unrelated input: %q", got)
	}
}

func contains(s, substr string) bool {
	return len(substr) == 0 || (len(s) >= len(substr) && index(s, substr) >= 0)
}

func index(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
