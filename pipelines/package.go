package pipelines

import (
	"context"
	"fmt"
	"path"

	"dagger.io/dagger"
	"github.com/grafana/grafana-build/containers"
	"github.com/grafana/grafana-build/executil"
)

// PackagedPaths are paths that are included in the grafana tarball.
var PackagedPaths = []string{
	"bin/",
	"conf/",
	"LICENSE",
	"NOTICE.md",
	"plugins-bundled/",
	"public/",
	"README.md",
	"VERSION",
	"docs/sources/",
	"packaging/deb",
	"packaging/rpm",
	"packaging/wrappers",
	"packaging/autocomplete",
}

type PackageOpts struct {
	Src          *dagger.Directory
	Version      string
	BuildID      string
	Distros      []executil.Distribution
	IsEnterprise bool
}

// PackageFile builds and packages Grafana into a tar.gz for each dsitrbution and returns a map of the dagger file that holds each tarball, keyed by the distribution it corresponds to.
func PackageFiles(ctx context.Context, d *dagger.Client, opts PackageOpts) (map[executil.Distribution]*dagger.File, error) {
	backends, err := GrafanaBackendBuildDirectories(ctx, d, opts.Src, opts.Distros, opts.Version)
	if err != nil {
		return nil, err
	}

	// Get the node version from .nvmrc...
	nodeVersion, err := containers.NodeVersion(d, opts.Src).Stdout(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get node version from source code: %w", err)
	}

	nodeCache := d.CacheVolume("node")
	nodeModules := containers.NodeModules(d, opts.Src, nodeCache, nodeVersion)
	if err != nil {
		return nil, err
	}

	frontend, err := GrafanaFrontendBuildDirectory(ctx, d, opts.Src, nodeModules, nodeVersion)
	if err != nil {
		return nil, err
	}

	plugins, err := containers.BuildPlugins(ctx, d, opts.Src, "plugins-bundled/internal", nodeModules, nodeVersion)
	if err != nil {
		return nil, err
	}

	packages := make(map[executil.Distribution]*dagger.File, len(backends))
	for k, backend := range backends {
		packager := d.Container().
			From(containers.BusyboxImage).
			WithMountedDirectory("/src", opts.Src).
			WithMountedDirectory("/src/bin", backend).
			WithMountedDirectory("/src/public", frontend).
			WithWorkdir("/src")

		for _, v := range plugins {
			packager = packager.WithMountedDirectory(path.Join("/src/plugins-bundled/internal", v.Name), v.Directory)
		}
		opts := TarFileOpts{
			Version:      opts.Version,
			BuildID:      opts.BuildID,
			IsEnterprise: opts.IsEnterprise,
			Distro:       k,
		}

		name := TarFilename(opts)
		packager = packager.WithExec([]string{"/bin/sh", "-c", fmt.Sprintf("echo \"%s\" > VERSION", opts.Version)}).
			WithExec(append([]string{"tar", "-czf", name}, PackagedPaths...))
		packages[k] = packager.File(name)
	}

	return packages, nil
}

// Package builds and packages Grafana into a tar.gz for each distribution provided.
func Package(ctx context.Context, d *dagger.Client, opts PackageOpts) error {
	packages, err := PackageFiles(ctx, d, opts)
	if err != nil {
		return err
	}

	for k, file := range packages {
		opts := TarFileOpts{
			Version:      opts.Version,
			BuildID:      opts.BuildID,
			IsEnterprise: opts.IsEnterprise,
			Distro:       k,
		}
		name := TarFilename(opts)
		if _, err := file.Export(ctx, name); err != nil {
			return err
		}
	}
	return nil
}
