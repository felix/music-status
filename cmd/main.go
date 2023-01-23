package main

import (
	"flag"
	"log"

	"src.userspace.com.au/felix/mstatus"
	_ "src.userspace.com.au/felix/mstatus/plugins/lastfm"
	_ "src.userspace.com.au/felix/mstatus/plugins/listenbrainz"
	_ "src.userspace.com.au/felix/mstatus/plugins/mpd"
	_ "src.userspace.com.au/felix/mstatus/plugins/slack"
)

func main() {
	var (
		configPath string
		verbose    bool
	)
	flag.StringVar(&configPath, "config", ".music-status.conf", "Config file")
	flag.StringVar(&configPath, "c", ".music-status.conf", "Config file")
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

	// This blocks
	err = svc.Start()
	if err != nil {
		log.Fatal(err)
	}
}
