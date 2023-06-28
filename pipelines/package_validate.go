package pipelines

import (
	"context"
	"fmt"
	"strings"

	"dagger.io/dagger"
	"github.com/grafana/grafana-build/containers"
	"github.com/grafana/grafana-build/executil"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

// ValidatePackage downloads a package and validates from a Google Cloud Storage bucket.
func ValidatePackage(ctx context.Context, d *dagger.Client, src *dagger.Directory, args PipelineArgs) error {
	packages, err := containers.GetPackages(ctx, d, args.PackageInputOpts, args.GCPOpts)
	if err != nil {
		return err
	}

	// Define all of the containers first, and where their artifacts will be exported to
	dirs := map[string]*dagger.Directory{}
	for i, name := range args.PackageInputOpts.Packages {
		pkg := packages[i]

		var dir *dagger.Directory
		if strings.HasSuffix(name, ".tar.gz") {
			dir, err = validatePackage(ctx, d, pkg, src, name)
			if err != nil {
				return err
			}
		} else if strings.HasSuffix(name, ".deb") {
			dir, err = validateDeb(ctx, d, pkg, src, name)
			if err != nil {
				return err
			}
		} else {
			return fmt.Errorf("unknown package extension")
		}

		// replace .tar.gz with .e2e-artifacts/
		destination := DestinationName(name, "e2e-artifacts")
		dirs[destination] = dir
	}

	var (
		grp = &errgroup.Group{}
		sm  = semaphore.NewWeighted(args.ConcurrencyOpts.Parallel)
	)

	// Run them in parallel
	for k, dir := range dirs {
		// Join the produced destination with the protocol given by the '--destination' flag.
		dst := strings.Join([]string{args.PublishOpts.Destination, k}, "/")
		grp.Go(PublishDirFunc(ctx, sm, d, dir, args.GCPOpts, dst))
	}
	return grp.Wait()
}

// validateDeb uses the given package (deb) and grafana source code (src) to run the e2e smoke tests.
// the returned directory is the e2e artifacts created by cypress (screenshots and videos).
func validateDeb(ctx context.Context, d *dagger.Client, deb *dagger.File, src *dagger.Directory, packageName string) (*dagger.Directory, error) {
	nodeVersion, err := containers.NodeVersion(d, src).Stdout(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get node version from source code: %w", err)
	}

	// This grafana service runs in the background for the e2e tests
	service := d.Container().From("debian:latest").
		WithFile("/src/package.deb", deb).
		WithExec([]string{"apt-get", "update"}).
		WithExec([]string{"apt-get", "install", "-y", "/src/package.deb"}).
		WithWorkdir("/usr/share/grafana").
		WithExec([]string{"grafana-server"}).
		WithExposedPort(3000)

	return containers.ValidatePackage(d, service, src, nodeVersion), nil
}

// validatePackage uses the given package (pkg) and grafana source code (src) to run the e2e smoke tests.
// the returned directory is the e2e artifacts created by cypress (screenshots and videos).
func validatePackage(ctx context.Context, d *dagger.Client, pkg *dagger.File, src *dagger.Directory, packageName string) (*dagger.Directory, error) {
	nodeVersion, err := containers.NodeVersion(d, src).Stdout(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get node version from source code: %w", err)
	}

	taropts := TarOptsFromFileName(packageName)

	// This grafana service runs in the background for the e2e tests
	service := d.Container(dagger.ContainerOpts{
		Platform: executil.Platform(taropts.Distro),
	}).From("alpine:latest").
		WithDirectory("/src", containers.ExtractedArchive(d, pkg, packageName)).
		WithWorkdir("/src").
		WithExec([]string{"./bin/grafana", "server"}).
		WithExposedPort(3000)

	return containers.ValidatePackage(d, service, src, nodeVersion), nil
}
