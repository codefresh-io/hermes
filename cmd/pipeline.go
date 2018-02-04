package main

import (
	"errors"
	"fmt"

	"github.com/codefresh-io/hermes/pkg/backend"
	"github.com/codefresh-io/hermes/pkg/codefresh"
	"github.com/urfave/cli"
)

var pipelineCommand = cli.Command{
	Name:  "pipeline",
	Usage: "setup Codefresh triggers, linking trigger events and pipelines",
	Subcommands: []cli.Command{
		{
			Name: "list",
			Flags: []cli.Flag{
				cli.StringSliceFlag{
					Name:  "event",
					Usage: "trigger event filter",
				},
			},
			Usage:       "list pipelines with triggers",
			Description: "List Codefresh pipelines that have triggers defined",
			Action:      listPipelines,
		},
		{
			Name:        "link",
			Usage:       "connect pipelines to the specified trigger event(s)",
			ArgsUsage:   "<pipeline-uid> <event-uri> [event-uri...]",
			Description: "Create a new trigger, linking a pipeline to the specified trigger event(s)",
			Action:      addTriggerPipelines,
		},
		{
			Name:        "unlink",
			Usage:       "disconnect pipeline from the specified trigger event(s)",
			ArgsUsage:   "<pipeline-uid> <event-uri> [event-uri...]",
			Description: "Delete pipeline trigger, by removing link between the pipeline and the specified trigger event(s)",
			Action:      deleteTriggerPipeline,
		},
	},
}

func listPipelines(c *cli.Context) error {
	triggerReaderWriter := backend.NewRedisStore(c.GlobalString("redis"), c.GlobalInt("redis-port"), c.GlobalString("redis-password"), nil)
	pipelines, err := triggerReaderWriter.GetPipelines(c.StringSlice("event"))
	if err != nil {
		return err
	}
	for _, p := range pipelines {
		fmt.Println(p)
	}

	return nil
}

func addTriggerPipelines(c *cli.Context) error {
	// get trigger name and pipeline
	args := c.Args()
	if len(args) != 2 {
		return errors.New("wrong number of arguments")
	}
	// get codefresh endpoint
	codefreshService := codefresh.NewCodefreshEndpoint(c.GlobalString("c"), c.GlobalString("t"))
	// get trigger service
	triggerReaderWriter := backend.NewRedisStore(c.GlobalString("redis"), c.GlobalInt("redis-port"), c.GlobalString("redis-password"), codefreshService)
	// create pipelines
	pipelines := make([]string, 1)
	pipelines[0] = args.Get(1)
	return triggerReaderWriter.AddPipelines(args.First(), pipelines)
}

func deleteTriggerPipeline(c *cli.Context) error {
	// get trigger name and pipeline
	args := c.Args()
	if len(args) != 2 {
		return errors.New("wrong number of arguments")
	}
	// get trigger service
	triggerReaderWriter := backend.NewRedisStore(c.GlobalString("redis"), c.GlobalInt("redis-port"), c.GlobalString("redis-password"), nil)
	// delete pipelines
	return triggerReaderWriter.DeletePipeline(c.Args().First(), args.Get(1))
}
