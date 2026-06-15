package cp

import (
	"fmt"
	"os"
	"strings"
)

// Target describes how to reach a Volcengine Continuous Delivery (CP) endpoint.
// The signing algorithm is identical across targets; only Host and Region (and
// thus the credential scope) differ.
type Target struct {
	// Name is the canonical target name (e.g. "prod-cn").
	Name string
	// Host is the OpenAPI gateway host, e.g. "open.volcengineapi.com".
	Host string
	// Region is the credential-scope region, e.g. "cn-beijing".
	Region string
	// Service is the credential-scope service, e.g. "cp".
	Service string
	// Version is the RPC API version query parameter.
	Version string
}

// DefaultService is the CP OpenAPI service name used in the credential scope.
// It can be overridden per target or via VOLC_CP_SERVICE.
const DefaultService = "cp"

// DefaultVersion is the CP OpenAPI version. It can be overridden per target or
// via VOLC_CP_VERSION without code changes.
const DefaultVersion = "2023-05-01"

var builtinTargets = map[string]Target{
	"pre": {
		Name: "pre", Host: "open.volcengineapi.com", Region: "cn-beijing",
		Service: DefaultService, Version: DefaultVersion,
	},
	"prod-cn": {
		Name: "prod-cn", Host: "open.volcengineapi.com", Region: "cn-beijing",
		Service: DefaultService, Version: DefaultVersion,
	},
	"prod-sg": {
		Name: "prod-sg", Host: "open.ap-southeast-1.volcengineapi.com", Region: "ap-southeast-1",
		Service: DefaultService, Version: DefaultVersion,
	},
	"byteplus-sg": {
		Name: "byteplus-sg", Host: "open.byteplusapi.com", Region: "ap-singapore-1",
		Service: DefaultService, Version: DefaultVersion,
	},
}

// ResolveTarget returns the Target for name, applying environment overrides.
// "prod" is an alias for "prod-cn". Overrides: VOLC_CP_ENDPOINT (host),
// VOLC_CP_REGION, VOLC_CP_SERVICE, VOLC_CP_VERSION.
func ResolveTarget(name string) (Target, error) {
	key := strings.ToLower(strings.TrimSpace(name))
	if key == "" || key == "prod" {
		key = "prod-cn"
	}
	target, ok := builtinTargets[key]
	if !ok {
		return Target{}, fmt.Errorf("cp: unknown target %q", name)
	}
	if v := os.Getenv("VOLC_CP_ENDPOINT"); v != "" {
		target.Host = strings.TrimPrefix(strings.TrimPrefix(v, "https://"), "http://")
	}
	if v := os.Getenv("VOLC_CP_REGION"); v != "" {
		target.Region = v
	}
	if v := os.Getenv("VOLC_CP_SERVICE"); v != "" {
		target.Service = v
	}
	if v := os.Getenv("VOLC_CP_VERSION"); v != "" {
		target.Version = v
	}
	return target, nil
}
