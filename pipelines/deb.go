package pipelines

import (
	"context"

	"dagger.io/dagger"
	"github.com/grafana/grafana-build/containers"
)

// Deb uses the grafana package given by the '--package' argument and creates a .deb installer.
// It accepts publish args, so you can place the file in a local or remote destination.
func Deb(ctx context.Context, d *dagger.Client, args PipelineArgs) error {
	return PackageInstaller(ctx, d, args, InstallerOpts{
		PackageType: "deb",
		ConfigFiles: [][]string{
			{"/src/packaging/deb/default/grafana-server", "/pkg/etc/default/grafana-server"},
			{"/src/packaging/deb/init.d/grafana-server", "/pkg/etc/init.d/grafana-server"},
			{"/src/packaging/deb/systemd/grafana-server.service", "/pkg/usr/lib/systemd/system/grafana-server.service"},
		},
		AfterInstall: "/src/packaging/deb/control/postinst",
		BeforeRemove: "/src/packaging/deb/control/prerm",
		Depends: []string{
			"adduser",
			"libfontconfig1",
		},
		ExtraArgs: []string{
			"--deb-no-default-config-files",
		},
		EnvFolder: "/pkg/etc/default",
		Container: containers.FPMContainer(d),
	})
}
