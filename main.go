package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/docker/docker/api/types/events"
	"log"
	"net/smtp"
	"os"
	"os/signal"
	"time"
)

type NotifierConfig struct {
	FromAddress string
	ToAddress   string

	Password string
	Host     string
	Port     string

	RestartCount    int
	RestartDuration time.Duration
	ViolationLimit  time.Duration
}

func (c *NotifierConfig) Address() string {
	return c.Host + ":" + c.Port
}

var config *NotifierConfig

func init() {
	config = &NotifierConfig{}

	flag.StringVar(&config.FromAddress, "from", "", "Email address to send from")
	flag.StringVar(&config.ToAddress, "to", "", "Email address to send to")
	flag.StringVar(&config.Password, "password", "", "Password")
	flag.StringVar(&config.Host, "host", "smtp.gmail.com", "SMTP server host")
	flag.StringVar(&config.Port, "port", "587", "SMTP server port")
	flag.IntVar(&config.RestartCount, "count", 5, "How many times a container must restart in a given period")
	flag.DurationVar(&config.RestartDuration, "window", time.Minute*5, "Time period in which restarts must occur")
	flag.DurationVar(&config.ViolationLimit, "limit", time.Hour*1, "Maximum duration between sending emails about a specific container")
	flag.Parse()

	if config.FromAddress == "" {
		log.Fatalln("-from argument must be set")
	}
	if config.ToAddress == "" {
		log.Fatalln("-to argument must be set")
	}
	if config.Host == "" {
		log.Fatalln("-host argument must be set")
	}
	if config.Port == "" {
		log.Fatalln("-port argument must be set")
	}
}

func signalWaiter(cancelFunc context.CancelFunc) {
	signals := make(chan os.Signal, 1)

	signal.Notify(signals)
	go func() {
		sig := <-signals
		log.Printf("Signal: %v\n", sig)
		cancelFunc()
	}()
}

func formatEmailMessage(event events.Message) string {
	header := fmt.Sprintf("To: %s\r\nSubject: Container Restart Notification\r\n", config.ToAddress)
	body := fmt.Sprintf("Container %q (%v) has restarted %d times in the past %v", event.Actor.Attributes["name"], event.Actor.ID, config.RestartCount, config.RestartDuration)
	return header + "\r\n" + body
}

func main() {
	ctx, cancelCtx := context.WithCancel(context.Background())
	signalWaiter(cancelCtx)

	tracker, err := NewRestartTracker(config.RestartDuration, config.RestartCount, config.ViolationLimit)
	if err != nil {
		panic(err)
	}

	smtpAuth := smtp.PlainAuth("", config.FromAddress, config.Password, config.Host)

	tracker.OnViolation = func(event events.Message, eventLog *eventTimeLog) {
		log.Printf("Violation %v %v\n", event.Actor.ID, event.Actor.Attributes["name"])
		msg := formatEmailMessage(event)
		err := smtp.SendMail(config.Address(), smtpAuth, config.FromAddress, []string{config.ToAddress}, []byte(msg))
		if err != nil {
			log.Fatalf("failed to send email: %v", err)
		}
	}

	tracker.OnError = func(err error) {
		log.Printf("Error: %v\n", err)
	}

	tracker.Run(ctx)
}
