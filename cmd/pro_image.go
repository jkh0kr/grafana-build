package main

import (
	"github.com/grafana/grafana-build/pipelines"
	"github.com/urfave/cli/v2"
)

var ProImageCommand = &cli.Command{
	Name:        "pro-image",
	Action:      PipelineAction(pipelines.ProImage),
	Description: "Creates a hosted grafana pro image",
	Flags:       JoinFlagsWithDefault(ProImageFlags),
}
