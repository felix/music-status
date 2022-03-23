package main

import (
	"encoding/csv"
	"flag"
	"log"
	"os"
	"strconv"
	"strings"

	"src.userspace.com.au/felix/mstatus"
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

	slack, err := slack.New(
		cfgString(cfg, "slackToken"),
		cfgString(cfg, "slackURL"),
		slack.DefaultStatus(cfgString(cfg, "defaultText")),
		slack.DefaultEmoji(cfgString(cfg, "defaultEmoji")),
		slack.ExpireAfter(cfgString(cfg, "expireStatus")),
		slack.Logger(logger),
	)
	if err != nil {
		log.Fatal(err)
	}

	mpd, err := mpd.New(
		mpd.Host(cfgString(cfg, "mpdHost")),
		mpd.Port(cfgInt(cfg, "mpdPort")),
		mpd.Password(cfgString(cfg, "mpdPassword")),
		mpd.Logger(logger),
	)
	if err != nil {
		log.Fatal(err)
	}

	// This blocks
	err = mpd.Watch([]mstatus.Publisher{slack})
	if err != nil {
		log.Fatal(err)
	}
}

func cfgString(cfg [][]string, key string) string {
	for _, row := range cfg {
		if strings.EqualFold(row[0], key) {
			return row[1]
		}
	}
	return ""
}

func cfgInt(cfg [][]string, key string) int {
	var out int
	for _, row := range cfg {
		if strings.EqualFold(row[0], key) {
			out, _ = strconv.Atoi(row[1])
		}
	}
	return out
}
