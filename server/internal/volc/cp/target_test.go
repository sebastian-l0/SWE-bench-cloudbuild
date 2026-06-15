package cp

import (
	"testing"
)

func TestResolveTargetProdAlias(t *testing.T) {
	got, err := ResolveTarget("prod")
	if err != nil {
		t.Fatalf("ResolveTarget: %v", err)
	}
	if got.Name != "prod-cn" || got.Region != "cn-beijing" || got.Host != "open.volcengineapi.com" {
		t.Fatalf("prod alias = %#v", got)
	}
}

func TestResolveTargetEmptyDefaultsToProdCN(t *testing.T) {
	got, err := ResolveTarget("")
	if err != nil {
		t.Fatalf("ResolveTarget: %v", err)
	}
	if got.Name != "prod-cn" {
		t.Fatalf("empty target = %q", got.Name)
	}
}

func TestResolveTargetBuiltins(t *testing.T) {
	cases := map[string]struct{ host, region string }{
		"pre":         {"open.volcengineapi.com", "cn-beijing"},
		"prod-cn":     {"open.volcengineapi.com", "cn-beijing"},
		"prod-sg":     {"open.ap-southeast-1.volcengineapi.com", "ap-southeast-1"},
		"byteplus-sg": {"open.byteplusapi.com", "ap-singapore-1"},
	}
	for name, want := range cases {
		got, err := ResolveTarget(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got.Host != want.host || got.Region != want.region {
			t.Fatalf("%s = %#v, want host=%s region=%s", name, got, want.host, want.region)
		}
	}
}

func TestResolveTargetUnknown(t *testing.T) {
	if _, err := ResolveTarget("does-not-exist"); err == nil {
		t.Fatal("expected error for unknown target")
	}
}

func TestResolveTargetEnvOverrides(t *testing.T) {
	t.Setenv("VOLC_CP_ENDPOINT", "https://custom.example.com")
	t.Setenv("VOLC_CP_REGION", "custom-region")
	t.Setenv("VOLC_CP_SERVICE", "custom-svc")
	t.Setenv("VOLC_CP_VERSION", "2099-01-01")
	got, err := ResolveTarget("prod-cn")
	if err != nil {
		t.Fatalf("ResolveTarget: %v", err)
	}
	if got.Host != "custom.example.com" || got.Region != "custom-region" ||
		got.Service != "custom-svc" || got.Version != "2099-01-01" {
		t.Fatalf("overrides not applied: %#v", got)
	}
}
