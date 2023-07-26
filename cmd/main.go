package main

import (
	"flag"
	"log"
	"os"
	"os/signal"

	"src.userspace.com.au/felix/mstatus"
	_ "src.userspace.com.au/felix/mstatus/plugins/lastfm"
	_ "src.userspace.com.au/felix/mstatus/plugins/listenbrainz"
	_ "src.userspace.com.au/felix/mstatus/plugins/mpd"
	_ "src.userspace.com.au/felix/mstatus/plugins/slack"
	_ "src.userspace.com.au/felix/mstatus/plugins/spotify"
)

func main() {
	var (
		configPath  string
		listPlugins bool
		verbose     bool
	)
	flag.StringVar(&configPath, "config", ".music-status.conf", "Config file")
	flag.StringVar(&configPath, "c", ".music-status.conf", "Config file")
	flag.BoolVar(&listPlugins, "plugins", false, "List plugins")
	flag.BoolVar(&verbose, "verbose", false, "Be verbose")
	flag.BoolVar(&verbose, "v", false, "Be verbose")
	flag.Parse()

	logger := func(...interface{}) {}
	if verbose {
		logger = log.Println
	}
	logger("being verbose")

	cfg, err := mstatus.ReadConfig(configPath)
	if err != nil {
		log.Fatal(err)
	}

	svc, err := mstatus.New(cfg, mstatus.WithLogger(logger))
	if err != nil {
		log.Fatal(err)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)

	go func() {
		<-sigs
		svc.Stop()
	}()

	err = svc.Start()
	if err != nil {
		log.Fatal(err)
	}

}
