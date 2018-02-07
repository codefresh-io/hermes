package main

import (
	"errors"
	"fmt"

	"github.com/codefresh-io/hermes/pkg/backend"
	"github.com/codefresh-io/hermes/pkg/codefresh"
	"github.com/urfave/cli"
)

var triggerEventCommand = cli.Command{
	Name:  "trigger-event",
	Usage: "manage trigger events",
	Subcommands: []cli.Command{
		{
			Name: "list",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "type",
					Usage: "trigger event type",
				},
				cli.StringFlag{
					Name:  "kind",
					Usage: "trigger event kind",
				},
			},
			Usage:       "list defined trigger events",
			Description: "List trigger events",
			Action:      listEvents,
		},
		{
			Name:        "get",
			Usage:       "get trigger event by event URI",
			ArgsUsage:   "<event-uri>",
			Description: "Get single trigger event",
			Action:      getEvent,
		},
		{
			Name: "create",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "type",
					Usage: "trigger event type",
				},
				cli.StringFlag{
					Name:  "kind",
					Usage: "trigger event kind",
				},
				cli.StringFlag{
					Name:  "secret",
					Usage: "trigger event secret (auto-generated when skipped)",
					Value: "!generate",
				},
				cli.StringSliceFlag{
					Name:  "value",
					Usage: "trigger event value pair(s); as defined by trigger type config",
				},
				cli.StringFlag{
					Name:  "credentials",
					Usage: "credentials for external system, required to subscribe to event (JSON)",
				},
			},
			Usage:       "create trigger event",
			Description: "Create/define trigger event",
			Action:      createEvent,
		},
		{
			Name: "delete",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "type",
					Usage: "trigger event type",
				},
				cli.StringFlag{
					Name:  "kind",
					Usage: "trigger event kind",
				},
				cli.StringFlag{
					Name:  "credentials",
					Usage: "credentials for external system, required to unsubscribe from event (JSON)",
				},
			},
			Usage:       "delete trigger event",
			ArgsUsage:   "<event-uri>",
			Description: "Delete/undefine trigger event by event URI",
			Action:      deleteEvent,
		},
		{
			Name:        "link",
			Usage:       "connect trigger event to the specified pipeline(s)",
			ArgsUsage:   "<event-uri> <pipeline> [pipeline...]",
			Description: "Create a new trigger, linking a trigger event to the specified pipeline(s)",
			Action:      linkEvent,
		},
		{
			Name:        "unlink",
			Usage:       "disconnect trigger event from the specified pipeline(s)",
			ArgsUsage:   "<event-uri> <pipeline> [pipeline...]",
			Description: "Delete trigger, by removing link between the trigger event and the specified pipeline(s)",
			Action:      unlinkEvent,
		},
	},
}

func listEvents(c *cli.Context) error {
	// get trigger backend
	triggerReaderWriter := backend.NewRedisStore(c.GlobalString("redis"), c.GlobalInt("redis-port"), c.GlobalString("redis-password"), nil, nil)
	// get trigger events
	events, err := triggerReaderWriter.GetEvents(c.String("type"), c.String("kind"), c.String("filter"))
	if err != nil {
		return err
	}
	for _, event := range events {
		fmt.Println(event)
	}
	return nil
}

func getEvent(c *cli.Context) error {
	// get trigger backend
	triggerReaderWriter := backend.NewRedisStore(c.GlobalString("redis"), c.GlobalInt("redis-port"), c.GlobalString("redis-password"), nil, nil)
	// get trigger events
	event, err := triggerReaderWriter.GetEvent(c.String("event"))
	if err != nil {
		return err
	}
	fmt.Println(event)
	return nil
}

func createEvent(c *cli.Context) error {
	// get event provider informer
	eventProviderInformer := backend.NewEventProviderManager(c.GlobalString("config"), c.GlobalBool("skip-monitor"))
	// get trigger backend
	triggerReaderWriter := backend.NewRedisStore(c.GlobalString("redis"), c.GlobalInt("redis-port"), c.GlobalString("redis-password"), nil, eventProviderInformer)
	// create new event
	event, err := triggerReaderWriter.CreateEvent(c.String("type"), c.String("kind"), c.String("secret"), nil)
	if err != nil {
		return err
	}
	// print it out
	fmt.Println("New trigger event successfully created.")
	fmt.Println(event.URI)

	return nil
}

func deleteEvent(c *cli.Context) error {
	// get trigger backend
	triggerReaderWriter := backend.NewRedisStore(c.GlobalString("redis"), c.GlobalInt("redis-port"), c.GlobalString("redis-password"), nil, nil)
	// get trigger events
	err := triggerReaderWriter.DeleteEvent(c.String("event"))
	if err != nil {
		return err
	}
	fmt.Println("Trigger event successfully deleted.")
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
	triggerReaderWriter := backend.NewRedisStore(c.GlobalString("redis"), c.GlobalInt("redis-port"), c.GlobalString("redis-password"), codefreshService, nil)
	// create triggers for event linking it to passed pipeline(s)
	return triggerReaderWriter.CreateTriggersForEvent(c.Args().First(), c.Args().Tail())
}

func unlinkEvent(c *cli.Context) error {
	// get trigger name and pipeline
	args := c.Args()
	if len(args) != 2 {
		return errors.New("wrong number of arguments")
	}
	// get trigger service
	triggerReaderWriter := backend.NewRedisStore(c.GlobalString("redis"), c.GlobalInt("redis-port"), c.GlobalString("redis-password"), nil, nil)
	// delete pipelines
	return triggerReaderWriter.DeleteTriggersForEvent(args.First(), args.Tail())
}
