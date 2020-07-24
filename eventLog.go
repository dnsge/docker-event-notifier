package main

import (
	"context"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"time"
)

type eventTimeLog struct {
	triggers      []time.Time
	lastViolation time.Time
}

type eventLogMap map[string]*eventTimeLog

func NewEventTimeLog() *eventTimeLog {
	return &eventTimeLog{
		triggers:      make([]time.Time, 0),
		lastViolation: time.Unix(0, 0),
	}
}

func (e *eventTimeLog) AppendNow() {
	e.triggers = append(e.triggers, time.Now())
}

func (e *eventTimeLog) EventsInPastDuration(duration time.Duration) int {
	if duration > 0 {
		duration = -duration
	}

	afterTime := time.Now().Add(duration)
	count := 0
	for _, t := range e.triggers {
		if t.After(afterTime) {
			count++
		}
	}

	return count
}

type ViolationHandler func(event events.Message, eventLog *eventTimeLog)

type RestartTracker struct {
	client      *EventClient
	eventLogMap eventLogMap

	window         time.Duration
	count          int
	violationLimit time.Duration

	OnViolation ViolationHandler
	OnError     ErrorHandler
}

func NewRestartTracker(window time.Duration, count int, violationLimit time.Duration) (*RestartTracker, error) {
	client, err := NewEnvEventClient()
	if err != nil {
		return nil, err
	}

	tracker := &RestartTracker{
		client:         client,
		eventLogMap:    make(eventLogMap),
		window:         window,
		count:          count,
		violationLimit: violationLimit,
		OnViolation:    func(event events.Message, eventLog *eventTimeLog) {},
		OnError:        func(err error) {},
	}

	client.EventHandler = func(event events.Message) {
		if event.Action != "restart" && event.Action != "start" {
			return
		}

		actor := event.Actor
		if _, exists := tracker.eventLogMap[actor.ID]; !exists {
			tracker.eventLogMap[actor.ID] = NewEventTimeLog()
		}
		eventLog := tracker.eventLogMap[actor.ID]
		eventLog.AppendNow()

		if eventLog.EventsInPastDuration(tracker.window) >= tracker.count {
			if time.Now().Sub(eventLog.lastViolation) > tracker.violationLimit {
				eventLog.lastViolation = time.Now()
				tracker.OnViolation(event, eventLog)
			}
		}
	}

	client.ErrorHandler = func(err error) {
		tracker.OnError(err)
	}

	return tracker, nil
}

func (e *RestartTracker) Run(ctx context.Context) {
	args := filters.NewArgs()
	args.Add("type", "container")

	e.client.runEventLoop(ctx, types.EventsOptions{
		Filters: args,
	})
}
