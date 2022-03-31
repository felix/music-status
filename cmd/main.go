package main

import (
	"encoding/csv"
	"flag"
	"log"
	"os"
	"strconv"
	"strings"

	"src.userspace.com.au/felix/mstatus"
	"src.userspace.com.au/felix/mstatus/listenbrainz"
	"src.userspace.com.au/felix/mstatus/mpd"
	"src.userspace.com.au/felix/mstatus/slack"
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

	f, err := os.Open(configPath)
	if err != nil {
		log.Fatalf("failed to read config %q: %s\n", configPath, err)
	}
	r := csv.NewReader(f)
	r.Comma = '='
	r.Comment = '#'

	cfg, err := r.ReadAll()
	if err != nil {
		log.Fatal(err)
	}

	logger := func(...interface{}) {}
	if verbose {
		logger = log.Println
	}

	var options []mstatus.Option

	targets := cfgString(cfg, "global", "targets")

	if strings.Contains(targets, "slack") {
		slack, err := slack.New(
			cfgString(cfg, "slack", "token"),
			cfgString(cfg, "slack", "url"),
			slack.DefaultStatus(cfgString(cfg, "slack", "defaultText")),
			slack.DefaultEmoji(cfgString(cfg, "slack", "defaultEmoji")),
			slack.ExpireAfter(cfgString(cfg, "slack", "expireStatus")),
			slack.Logger(logger),
		)
		if err != nil {
			log.Fatal(err)
		}
		options = append(options, mstatus.WithHandler(slack))
	}
	if strings.Contains(targets, "listenbrainz") {
		brainz, err := listenbrainz.New(
			cfgString(cfg, "listenbrainz", "token"),
			listenbrainz.Logger(logger),
		)
		if err != nil {
			log.Fatal(err)
		}
		options = append(options, mstatus.WithHandler(brainz))
	}

	mpd, err := mpd.New(
		mpd.Host(cfgString(cfg, "mpd", "host")),
		mpd.Port(cfgInt(cfg, "mpd", "port")),
		mpd.Password(cfgString(cfg, "mpd", "password")),
		mpd.Logger(logger),
	)
	if err != nil {
		log.Fatal(err)
	}

	svc, err := mstatus.New(mpd, options...)

	// This blocks
	err = svc.Start()
	if err != nil {
		log.Fatal(err)
	}
}

func cfgString(cfg [][]string, scope, key string) string {
	for _, row := range cfg {
		parts := strings.SplitN(row[0], ".", 2)
		if strings.EqualFold(parts[0], scope) && strings.EqualFold(parts[1], key) {
			return row[1]
		}
	}
	return ""
}

func cfgInt(cfg [][]string, scope, key string) int {
	var out int
	if s := cfgString(cfg, scope, key); s != "" {
		out, _ = strconv.Atoi(s)
	}
	return out
}
