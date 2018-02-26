package main

import (
	"errors"
	"fmt"

	"github.com/codefresh-io/hermes/pkg/backend"
	"github.com/codefresh-io/hermes/pkg/codefresh"
	"github.com/codefresh-io/hermes/pkg/model"
	"github.com/urfave/cli"
)

var triggerCommand = cli.Command{
	Name:  "trigger",
	Usage: "configure Codefresh triggers",
	Subcommands: []cli.Command{
		{
			Name: "list",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "event",
					Usage: "trigger event filter (cannot be mixed with 'pipeline')",
				},
				cli.StringFlag{
					Name:  "pipeline",
					Usage: "pipeline filter (cannot be mixed with 'event')",
				},
				cli.StringFlag{
					Name:  "account",
					Usage: "Codefresh account ID",
					Value: model.PublicAccount,
				},
			},
			Usage:       "list defined triggers",
			Description: "List triggers filtered by trigger event or pipeline",
			Action:      listTriggers,
		},
		{
			Name:        "link",
			Usage:       "connect trigger event to the specified pipeline",
			ArgsUsage:   "<event-uri> <pipeline> [pipeline...]",
			Description: "Create a new trigger, linking a trigger event to the specified pipeline",
			Action:      linkEvent,
		},
		{
			Name:        "unlink",
			Usage:       "disconnect trigger event from the specified pipeline",
			ArgsUsage:   "<event-uri> <pipeline>",
			Description: "Delete trigger, by removing link between the trigger event and the specified pipeline",
			Action:      unlinkEvent,
		},
	},
}

// get triggers by name(s), filter or ALL
func listTriggers(c *cli.Context) error {
	triggerReaderWriter := backend.NewRedisStore(c.GlobalString("redis"), c.GlobalInt("redis-port"), c.GlobalString("redis-password"), nil, nil)
	// get event or pipeline
	event := c.String("event")
	pipeline := c.String("pipeline")

	// triggers slice
	var err error
	var triggers []model.Trigger

	// list by event
	if event != "" {
		triggers, err = triggerReaderWriter.GetEventTriggers(getContext(c), event)
		if err != nil {
			return err
		}
	}

	// list by pipeline
	if pipeline != "" {
		triggers, err = triggerReaderWriter.GetPipelineTriggers(getContext(c), pipeline)
		if err != nil {
			return err
		}
	}

	if len(triggers) == 0 {
		return errors.New("no triggers defined")
	}
	for _, t := range triggers {
		fmt.Println(t)
	}
	return nil
}

func linkEvent(c *cli.Context) error {
	// get trigger name and pipeline
	args := c.Args()
	if len(args) < 2 {
		return errors.New("wrong number of arguments")
	}
	// get codefresh endpoint
	codefreshService := codefresh.NewCodefreshEndpoint(c.GlobalString("c"), c.GlobalString("t"))
	// get trigger service
	eventReaderWriter := backend.NewRedisStore(c.GlobalString("redis"), c.GlobalInt("redis-port"), c.GlobalString("redis-password"), codefreshService, nil)
	// create triggers for event linking it to passed pipeline(s)
	return eventReaderWriter.CreateTriggersForEvent(c.Args().First(), c.Args().Tail())
}

func unlinkEvent(c *cli.Context) error {
	// get trigger name and pipeline
	args := c.Args()
	if len(args) != 2 {
		return errors.New("wrong number of arguments")
	}
	// get trigger service
	eventReaderWriter := backend.NewRedisStore(c.GlobalString("redis"), c.GlobalInt("redis-port"), c.GlobalString("redis-password"), nil, nil)
	// delete pipelines
	return eventReaderWriter.DeleteTriggersForEvent(args.First(), args.Tail())
}
