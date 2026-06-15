package config

import (
	"os"
	"regexp"
	"strconv"
)

const defaultDatabaseURL = "postgres://swe_cloudbuild:swe_cloudbuild@localhost:5432/swe_cloudbuild?sslmode=disable"

type Config struct {
	HTTPAddr          string
	DatabaseURL       string
	VolcTarget        string
	TOS               TOSConfig
	Dataset           DatasetConfig
	Materializer      MaterializerConfig
	RegistryNamespace string
	Concurrency       ConcurrencyConfig
	CP                CPConfig
	MockMode          bool
}

type TOSConfig struct {
	Bucket     string
	ParentPath string
}

type DatasetConfig struct {
	Name  string
	Split string
}

type MaterializerConfig struct {
	RepoURL string
	Ref     string
}

type ConcurrencyConfig struct {
	Base     int
	Env      int
	Instance int
}

type CPConfig struct {
	WorkspacePrefix string
	PipelinePrefix  string
}

func Defaults() Config {
	return Config{
		HTTPAddr:    ":8080",
		DatabaseURL: defaultDatabaseURL,
		VolcTarget:  "prod-cn",
		TOS: TOSConfig{
			ParentPath: "swe-cloudbuild",
		},
		Dataset: DatasetConfig{
			Name:  "SWE-bench/SWE-bench",
			Split: "test",
		},
		Materializer: MaterializerConfig{
			RepoURL: "https://github.com/sebastian-l0/SWE-bench",
			Ref:     "feature/materialize-image-contexts",
		},
		Concurrency: ConcurrencyConfig{
			Base:     1,
			Env:      10,
			Instance: 20,
		},
		CP: CPConfig{
			WorkspacePrefix: "swe-cloudbuild",
			PipelinePrefix:  "swe-image-build",
		},
	}
}

func Load() Config {
	cfg := Defaults()
	cfg.HTTPAddr = envString("SWE_CLOUDBUILD_HTTP_ADDR", cfg.HTTPAddr)
	cfg.DatabaseURL = envString("DATABASE_URL", cfg.DatabaseURL)
	cfg.VolcTarget = envString("SWE_CLOUDBUILD_VOLC_TARGET", cfg.VolcTarget)
	cfg.TOS.Bucket = envString("SWE_CLOUDBUILD_TOS_BUCKET", cfg.TOS.Bucket)
	cfg.TOS.ParentPath = envString("SWE_CLOUDBUILD_TOS_PREFIX", cfg.TOS.ParentPath)
	cfg.Dataset.Name = envString("SWE_CLOUDBUILD_DATASET", cfg.Dataset.Name)
	cfg.Dataset.Split = envString("SWE_CLOUDBUILD_DATASET_SPLIT", cfg.Dataset.Split)
	cfg.Materializer.RepoURL = envString("SWE_CLOUDBUILD_MATERIALIZER_REPO", cfg.Materializer.RepoURL)
	cfg.Materializer.Ref = envString("SWE_CLOUDBUILD_MATERIALIZER_REF", cfg.Materializer.Ref)
	cfg.RegistryNamespace = envString("SWE_CLOUDBUILD_REGISTRY_NAMESPACE", cfg.RegistryNamespace)
	cfg.CP.WorkspacePrefix = envString("SWE_CLOUDBUILD_CP_WORKSPACE_PREFIX", cfg.CP.WorkspacePrefix)
	cfg.CP.PipelinePrefix = envString("SWE_CLOUDBUILD_CP_PIPELINE_PREFIX", cfg.CP.PipelinePrefix)
	cfg.Concurrency.Base = envInt("SWE_CLOUDBUILD_CONCURRENCY_BASE", cfg.Concurrency.Base)
	cfg.Concurrency.Env = envInt("SWE_CLOUDBUILD_CONCURRENCY_ENV", cfg.Concurrency.Env)
	cfg.Concurrency.Instance = envInt("SWE_CLOUDBUILD_CONCURRENCY_INSTANCE", cfg.Concurrency.Instance)
	cfg.MockMode = envBool("SWE_CLOUDBUILD_MOCK", cfg.MockMode) || envBool("VOLC_MOCK", false)
	return cfg
}

func envString(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func envBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

var sensitivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(AK|SK|ACCESS_KEY|SECRET_KEY|TOKEN|PASSWORD|DATABASE_URL)=[^\s]+`),
	regexp.MustCompile(`(?i)(Authorization:\s*)[^\s]+(?:\s+[^\s]+)?`),
	regexp.MustCompile(`postgres://[^\s]+`),
}

func Redact(input string) string {
	out := input
	for _, pattern := range sensitivePatterns {
		out = pattern.ReplaceAllStringFunc(out, func(match string) string {
			if len(match) >= 14 && (match[:14] == "Authorization:" || match[:14] == "authorization:") {
				return match[:14] + " <redacted>"
			}
			return "<redacted>"
		})
	}
	return out
}
