package main

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/client"
)

type EventHandler func(event events.Message)
type ErrorHandler func(err error)

type EventClient struct {
	*client.Client
	EventHandler EventHandler
	ErrorHandler ErrorHandler
}

func NewEnvEventClient() (*EventClient, error) {
	cli, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}
	return &EventClient{
		Client:       cli,
		EventHandler: func(event events.Message) {},
		ErrorHandler: func(err error) {},
	}, nil
}

func (cli *EventClient) runEventLoop(ctx context.Context, options types.EventsOptions) {
	eventsChan, errChan := cli.Events(ctx, options)

	fmt.Println("Beginning event loop")
	for {
		select {
		case event := <-eventsChan:
			cli.EventHandler(event)
		case err := <-errChan:
			cli.ErrorHandler(err)
			return
		case <-ctx.Done():
			return
		}
	}
}
