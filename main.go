package main

import (
	"flag"

	"fmt"
	"os"

	"os/signal"
	"syscall"

	"context"

	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/moby/moby/client"
	log "github.com/sirupsen/logrus"
)

const (
	SELENE_WATCH_LABEL = "co.selene.activate=true"
)

func checkContainer(c types.Container, cli *client.Client) {
	json, err := cli.ContainerInspect(context.Background(), c.ID)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
			"image": c.Image,
		}).Error("error getting details for container")
	} else {
		s := json.State.Health.Status

		log.WithFields(log.Fields{
			"image": c.Image,
			"state": s,
		}).Debug("current state")

		if s == types.Unhealthy {

			log.WithFields(log.Fields{
				"image": c.Image,
			}).Warn("unhealthy state. restarting.")

			var t time.Duration = 5 * time.Second

			cli.ContainerRestart(context.Background(), c.ID, &t)

		}
	}
}

func main() {
	logLevel := flag.String("log-level", "info", "log level to use for logging")
	interval := flag.Int("check-interval", 5, "interval to check for unhealthy containers")

	flag.Parse()

	lvl, err := log.ParseLevel(*logLevel)
	if err != nil {
		fmt.Printf("error setting log level : %s", err)
		os.Exit(1)
	}
	log.SetLevel(lvl)

	log.Info("starting")

	cli, err := client.NewEnvClient()
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("error creating docker client")
		os.Exit(1)
	}

	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		cli.Close()
		log.Info("caught shutdown signal. exiting.")
		os.Exit(0)
	}()

	args := filters.NewArgs()
	args.Add("label", SELENE_WATCH_LABEL)

	for {
		containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{Filters: args})

		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
			}).Error("error getting list of containers")
		}

		log.WithFields(log.Fields{
			"len": len(containers),
		}).Debug("starting containers check")

		for _, c := range containers {
			checkContainer(c, cli)
		}

		log.Debug("done containers check")

		time.Sleep(time.Duration(*interval) * time.Second)
	}

}
