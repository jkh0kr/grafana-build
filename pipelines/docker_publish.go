package pipelines

import (
	"context"
	"fmt"
	"log"
	"strings"

	"dagger.io/dagger"
	"github.com/grafana/grafana-build/containers"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

func ImageManifest(tag string) string {
	manifest := strings.ReplaceAll(tag, "-image-tags", "")
	lastDash := strings.LastIndex(manifest, "-")
	return manifest[:lastDash]
}

// DockerPublish is a pipeline that uses a grafana.docker.tar.gz as input and publishes a Docker image to a container registry or repository.
// Grafana's Dockerfile should support supplying a tar.gz using a --build-arg.
func DockerPublish(ctx context.Context, d *dagger.Client, args PipelineArgs) error {
	opts := args.DockerOpts
	packages, err := containers.GetPackages(ctx, d, args.PackageInputOpts, args.GCPOpts)
	if err != nil {
		return err
	}

	var (
		wg = &errgroup.Group{}
		sm = semaphore.NewWeighted(args.ConcurrencyOpts.Parallel)
	)

	manifestTags := make(map[string][]string)
	for i, name := range args.PackageInputOpts.Packages {
		// For each package we retrieve the tags grafana-image-tags and grafana-oss-image-tags, or grafana-enterprise-image-tags
		base := BaseImageAlpine
		tarOpts := TarOptsFromFileName(name)
		if strings.Contains(name, "ubuntu") {
			base = BaseImageUbuntu
		}

		tags := GrafanaImageTags(base, opts.Registry, tarOpts)
		for _, tag := range tags {
			// For each tag we publish an image and add the tag to the list of tags for a specific manifest
			// Since each package has a maximum of 2 tags, this for loop will only run twice on a worst case scenario
			manifest := ImageManifest(tag)
			manifestTags[manifest] = append(manifestTags[manifest], tag)
			wg.Go(PublishPackageImageFunc(ctx, sm, d, packages[i], tag, opts))
		}
	}

	if err := wg.Wait(); err != nil {
		// Wait for all images to be published
		return err
	}

	for manifest, tags := range manifestTags {
		// Publish each manifest
		wg.Go(PublishDockerManifestFunc(ctx, sm, d, manifest, tags, opts))
	}

	return wg.Wait()
}

func PublishPackageImageFunc(ctx context.Context, sm *semaphore.Weighted, d *dagger.Client, pkg *dagger.File, tag string, opts *containers.DockerOpts) func() error {
	return func() error {
		log.Printf("[%s] Attempting to publish image", tag)
		log.Printf("[%s] Acquiring semaphore", tag)
		if err := sm.Acquire(ctx, 1); err != nil {
			return fmt.Errorf("failed to acquire semaphore: %w", err)
		}
		defer sm.Release(1)
		log.Printf("[%s] Acquired semaphore", tag)

		log.Printf("[%s] Publishing image", tag)
		out, err := containers.PublishPackageImage(ctx, d, pkg, tag, opts)
		if err != nil {
			return fmt.Errorf("[%s] error: %w", tag, err)
		}
		log.Printf("[%s] Done publishing image", tag)

		fmt.Fprintln(Stdout, out)
		return nil
	}
}

func PublishDockerManifestFunc(ctx context.Context, sm *semaphore.Weighted, d *dagger.Client, manifest string, tags []string, opts *containers.DockerOpts) func() error {
	return func() error {
		log.Printf("[%s] Attempting to publish manifest", manifest)
		log.Printf("[%s] Acquiring semaphore", manifest)
		if err := sm.Acquire(ctx, 1); err != nil {
			return fmt.Errorf("failed to acquire semaphore: %w", err)
		}
		defer sm.Release(1)
		log.Printf("[%s] Acquired semaphore", manifest)

		log.Printf("[%s] Publishing manifest", manifest)
		out, err := containers.PublishDockerManifest(ctx, d, manifest, tags, opts)
		if err != nil {
			return fmt.Errorf("[%s] error: %w", manifest, err)
		}
		log.Printf("[%s] Done publishing manifest", manifest)

		fmt.Fprintln(Stdout, out)
		return nil
	}
}