// Package orchestrator drives a SWE-bench CloudBuild run: it prepares CP
// resources, then builds base -> env -> instance images under a strict layer
// gate, polling CP for status.
package orchestrator

import (
	"fmt"
	"strings"

	"github.com/sebastian-l0/SWE-bench-cloudbuild/server/internal/manifest"
	"github.com/sebastian-l0/SWE-bench-cloudbuild/server/internal/volc/cp"
)

// pipelineSpec is the parameterized CP pipeline definition shared by all three
// layers. The download step branches on $(parameters.type); the build step
// pushes to the registry repo with $(parameters.tag).
const pipelineSpec = `version: 1.0.0
agentPool: public/prod-v2-public
stages:
    - stage: stage-c1
      displayName: build
      tasks:
        - task: task-c2
          displayName: build-image
          timeout: 2h
          steps:
            - step: step-c4
              displayName: download-context
              component: execCmd@1.0.0/shell
              inputs:
                cmd: |-
                    #!/bin/bash
                    set -euo pipefail

                    REGISTRY="$(parameters.registry)"
                    NAMESPACE="$(parameters.namespace)"
                    REPO="$(parameters.repo)"
                    TAG="$(parameters.tag)"
                    TYPE="$(parameters.type)"
                    SCRIPT="$(parameters.script)"
                    TOSBUCKET="$(parameters.tosbucket)"
                    TOSREGION="$(parameters.region)"
                    TOSPATH="$(parameters.tospath)"

                    BASE_URL="https://${TOSBUCKET}.tos-${TOSREGION}.volces.com/${TOSPATH}/${TYPE}/${REGISTRY}_${NAMESPACE}_${REPO}_${TAG}"

                    download() {
                      echo ">>> downloading: $1"
                      wget --no-verbose --tries=3 --timeout=30 -O "$2" "$1"
                    }

                    case "${TYPE}" in
                      base)
                        download "${BASE_URL}/Dockerfile" "Dockerfile"
                        ;;
                      env|instance)
                        download "${BASE_URL}/Dockerfile" "Dockerfile"
                        download "${BASE_URL}/${SCRIPT}" "${SCRIPT}"
                        ;;
                      *)
                        echo "ERROR: unsupported type '${TYPE}' (expected: base | env | instance)" >&2
                        exit 1
                        ;;
                    esac

                    echo ">>> all downloads completed."
                shell: BASH
            - step: step-c1
              displayName: build-and-push
              component: build@2.0.0/buildkit-cr@5.0.0
              inputs:
                buildParams: ""
                compression: gzip
                contextPath: .
                disableSSLVerify: false
                dockerfiles:
                    default:
                        path: Dockerfile
                namespace: $(parameters.namespace)
                nydusify: true
                region: $(parameters.region)
                registryInstance: $(parameters.registryInstance)
                repo: $(parameters.repo)
                tag: $(parameters.tag)
                useCache: false
          outputs:
            - imageOutput_step-c1
          workspace: {}
          resourcesPolicy: all
          resources:
            limits:
                cpu: 2C
                memory: 4Gi
            storage:
                sizeLimit: ""
`

// BuildSettings holds the static configuration injected into pipeline parameters.
type BuildSettings struct {
	Registry         string // registry domain for the TOS download URL, e.g. agentkit-...cr.volces.com
	RegistryInstance string // CR instance name for the build step, e.g. agentkit-platform-2100483201
	Namespace        string
	Repo             string
	TOSBucket        string
	TOSRegion        string
	TOSPath          string // timestamp prefix path under the bucket (without /contexts)
}

// scriptForLayer returns the setup script parameter for a layer.
func scriptForLayer(layer string) string {
	switch layer {
	case manifest.LayerEnv:
		return "setup_env.sh"
	case manifest.LayerInstance:
		return "setup_repo.sh"
	default:
		return "none"
	}
}

// typeParam maps a layer to the CP "type" parameter, which must match the
// materializer context subdirectory name (instances is plural).
func typeParam(layer string) string {
	if layer == manifest.LayerInstance {
		return "instances"
	}
	return layer
}

// registryTag extracts the registry tag from a target image reference, i.e. the
// segment after the last colon (e.g. "...:sweb.base.py.x86_64" -> "sweb.base.py.x86_64").
// This already carries the materializer's "__" -> "_1776_" sanitization and
// matches the on-disk context directory suffix.
func registryTag(targetImage string) string {
	if idx := strings.LastIndex(targetImage, ":"); idx >= 0 {
		return targetImage[idx+1:]
	}
	return targetImage
}

// createPipelineInput builds the CreatePipelineInput for a layer's pipeline. The
// default parameter values use the layer; per-image overrides are supplied at
// RunPipeline time.
func createPipelineInput(workspaceID, name, layer string, s BuildSettings) cp.CreatePipelineInput {
	return cp.CreatePipelineInput{
		WorkspaceID: workspaceID,
		Name:        name,
		Description: fmt.Sprintf("SWE-bench %s images builder", layer),
		Spec:        pipelineSpec,
		Parameters: []cp.PipelineParameter{
			{Key: "namespace", Value: s.Namespace, Dynamic: true},
			{Key: "region", Value: s.TOSRegion, Dynamic: true},
			{Key: "registry", Value: s.Registry, Dynamic: true},
			{Key: "registryInstance", Value: s.RegistryInstance, Dynamic: true},
			{Key: "repo", Value: s.Repo, Dynamic: true},
			{Key: "script", Value: scriptForLayer(layer), Dynamic: true,
				OptionValues: []string{"none", "setup_env.sh", "setup_repo.sh"}},
			{Key: "tag", Value: "latest", Dynamic: true},
			{Key: "tosbucket", Value: s.TOSBucket, Dynamic: true},
			{Key: "tospath", Value: s.TOSPath, Dynamic: true},
			{Key: "type", Value: typeParam(layer), Dynamic: true,
				OptionValues: []string{"base", "env", "instances"}},
		},
	}
}

// runParams builds the per-image RunPipeline parameter overrides. The registry
// tag comes from the image's target reference (already sanitized to match the
// on-disk context directory), and the layer drives type/script.
func runParams(layer, targetImage string, s BuildSettings) []cp.RunPipelineParam {
	return []cp.RunPipelineParam{
		{Key: "namespace", Value: s.Namespace},
		{Key: "region", Value: s.TOSRegion},
		{Key: "registry", Value: s.Registry},
		{Key: "registryInstance", Value: s.RegistryInstance},
		{Key: "repo", Value: s.Repo},
		{Key: "script", Value: scriptForLayer(layer)},
		{Key: "tag", Value: registryTag(targetImage)},
		{Key: "tosbucket", Value: s.TOSBucket},
		{Key: "tospath", Value: s.TOSPath},
		{Key: "type", Value: typeParam(layer)},
	}
}
