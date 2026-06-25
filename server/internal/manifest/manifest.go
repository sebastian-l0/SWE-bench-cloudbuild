// Package manifest parses the manifest.json produced by the SWE-bench
// materializer (sebastian-l0/SWE-bench, feature/materialize-image-contexts) and
// normalizes it into a dependency-ordered set of image build records.
package manifest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Layer names matching the orchestration order.
const (
	LayerBase     = "base"
	LayerEnv      = "env"
	LayerInstance = "instance"
)

// rawManifest mirrors the on-disk manifest.json schema. The three image
// collections are JSON arrays.
type rawManifest struct {
	SchemaVersion string         `json:"schema_version"`
	DatasetName   string         `json:"dataset_name"`
	Split         string         `json:"split"`
	ImagePrefix   string         `json:"image_prefix"`
	Tag           string         `json:"tag"`
	Arch          string         `json:"arch"`
	Platform      string         `json:"platform"`
	BaseImages    []rawImage     `json:"base_images"`
	EnvImages     []rawImage     `json:"env_images"`
	InstanceImage []rawImage     `json:"instance_images"`
	Extra         map[string]any `json:"-"`
}

type rawImage struct {
	InstanceID        string `json:"instance_id"`
	LocalImageKey     string `json:"local_image_key"`
	TargetImage       string `json:"target_image"`
	ContextPath       string `json:"context_path"`
	Dockerfile        string `json:"dockerfile"`
	Platform          string `json:"platform"`
	Language          string `json:"language"`
	BaseImage         string `json:"base_image"`
	BaseLocalImageKey string `json:"base_local_image_key"`
	EnvImage          string `json:"env_image"`
	EnvLocalImageKey  string `json:"env_local_image_key"`
}

// Image is a normalized image build record.
type Image struct {
	Layer        string
	LocalKey     string
	TargetImage  string
	ContextPath  string
	Dockerfile   string
	DependsOnKey string
	InstanceID   string
	RawJSON      string
}

// Manifest is the parsed and validated manifest.
type Manifest struct {
	SchemaVersion string
	DatasetName   string
	Split         string
	ImagePrefix   string
	Tag           string
	Arch          string
	Platform      string
	Images        []Image // ordered base, then env, then instance
	RawJSON       string
}

// ParseFile reads and validates manifest.json from outputDir. context_path and
// dockerfile entries are validated to stay within outputDir.
func ParseFile(outputDir string) (*Manifest, error) {
	path := filepath.Join(outputDir, "manifest.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("manifest: read: %w", err)
	}
	return Parse(data, outputDir)
}

// Parse validates raw manifest bytes. When outputDir is non-empty, context and
// Dockerfile paths are checked for traversal outside it.
func Parse(data []byte, outputDir string) (*Manifest, error) {
	var raw rawManifest
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("manifest: decode: %w", err)
	}

	m := &Manifest{
		SchemaVersion: raw.SchemaVersion,
		DatasetName:   raw.DatasetName,
		Split:         raw.Split,
		ImagePrefix:   raw.ImagePrefix,
		Tag:           raw.Tag,
		Arch:          raw.Arch,
		Platform:      raw.Platform,
		RawJSON:       string(data),
	}

	if len(raw.BaseImages) == 0 {
		return nil, fmt.Errorf("manifest: no base_images")
	}

	keys := make(map[string]struct{})
	addImage := func(layer string, ri rawImage, dependsOn string) error {
		if ri.LocalImageKey == "" {
			return fmt.Errorf("manifest: %s image missing local_image_key", layer)
		}
		if _, dup := keys[ri.LocalImageKey]; dup {
			return fmt.Errorf("manifest: duplicate local_image_key %q", ri.LocalImageKey)
		}
		if ri.TargetImage == "" {
			return fmt.Errorf("manifest: %s image %q missing target_image", layer, ri.LocalImageKey)
		}
		if ri.ContextPath == "" {
			return fmt.Errorf("manifest: %s image %q missing context_path", layer, ri.LocalImageKey)
		}
		dockerfile := ri.Dockerfile
		if dockerfile == "" {
			dockerfile = "Dockerfile"
		}
		if err := checkContained(outputDir, ri.ContextPath); err != nil {
			return fmt.Errorf("manifest: %s image %q context_path: %w", layer, ri.LocalImageKey, err)
		}
		if err := checkContained(outputDir, filepath.Join(ri.ContextPath, dockerfile)); err != nil {
			return fmt.Errorf("manifest: %s image %q dockerfile: %w", layer, ri.LocalImageKey, err)
		}
		keys[ri.LocalImageKey] = struct{}{}
		rawEntry, _ := json.Marshal(ri)
		m.Images = append(m.Images, Image{
			Layer:        layer,
			LocalKey:     ri.LocalImageKey,
			TargetImage:  ri.TargetImage,
			ContextPath:  ri.ContextPath,
			Dockerfile:   dockerfile,
			DependsOnKey: dependsOn,
			InstanceID:   ri.InstanceID,
			RawJSON:      string(rawEntry),
		})
		return nil
	}

	for _, ri := range raw.BaseImages {
		if err := addImage(LayerBase, ri, ""); err != nil {
			return nil, err
		}
	}
	for _, ri := range raw.EnvImages {
		if err := addImage(LayerEnv, ri, ri.BaseLocalImageKey); err != nil {
			return nil, err
		}
	}
	for _, ri := range raw.InstanceImage {
		if err := addImage(LayerInstance, ri, ri.EnvLocalImageKey); err != nil {
			return nil, err
		}
	}

	// Validate dependency keys exist (when declared).
	for _, img := range m.Images {
		if img.DependsOnKey == "" {
			continue
		}
		if _, ok := keys[img.DependsOnKey]; !ok {
			return nil, fmt.Errorf("manifest: %s image %q depends on missing key %q",
				img.Layer, img.LocalKey, img.DependsOnKey)
		}
	}

	return m, nil
}

// checkContained ensures rel resolves inside base. When base is empty only
// absolute paths and parent traversal are rejected.
func checkContained(base, rel string) error {
	clean := filepath.Clean(rel)
	if filepath.IsAbs(clean) {
		return fmt.Errorf("absolute path not allowed: %q", rel)
	}
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path escapes output directory: %q", rel)
	}
	if base == "" {
		return nil
	}
	absBase, err := filepath.Abs(base)
	if err != nil {
		return err
	}
	target := filepath.Join(absBase, clean)
	relPath, err := filepath.Rel(absBase, target)
	if err != nil {
		return err
	}
	if relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path escapes output directory: %q", rel)
	}
	return nil
}
