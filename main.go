package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/fhs/gompd/mpd"
)

const statusMaxLength = 100

type setter func(interface{}) error

func newSetter(token, url string) setter {
	return func(p interface{}) error {
		uri := url + "/users.profile.set"

		buf := &bytes.Buffer{}
		enc := json.NewEncoder(buf)
		if err := enc.Encode(p); err != nil {
			return err
		}
		req, err := http.NewRequest("POST", uri, buf)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
		req.Header.Set("Content-Type", "application/json")
		_, err = http.DefaultClient.Do(req)
		return err
	}
}

func setStatus(slack setter, emoji, text string, expiry int64) error {
	if len(text) > statusMaxLength {
		text = text[:statusMaxLength-2] + "…"
	}

	log.Printf("Setting status %s %s\n", emoji, text)

	payload := map[string]struct {
		StatusText       string `json:"status_text"`
		StatusEmoji      string `json:"status_emoji"`
		StatusExpiration int64  `json:"status_expiration"`
	}{
		"profile": {
			StatusText:       text,
			StatusEmoji:      emoji,
			StatusExpiration: expiry,
		},
	}
	return slack(payload)
}

func main() {
	var (
		apiToken     = flag.String("slack-token", os.Getenv("SLACK_TOKEN"), "Slack API token")
		apiURL       = flag.String("slack-url", os.Getenv("SLACK_URL"), "Base URL for your Slack team")
		mpdAddress   = flag.String("mpd-address", "127.0.0.1:6600", "MPD address")
		mpdPassword  = flag.String("mpd-password", os.Getenv("MPD_PASSWORD"), "MPD password")
		defaultEmoji = flag.String("default-emoji", "", "Default status emoji")
		defaultText  = flag.String("default-text", "", "Default status text")
		expireStatus = flag.Bool("expire-status", false, "Set status expiry, approximately 30s")
	)
	flag.Parse()

	if !strings.HasPrefix(*apiURL, "http") {
		*apiURL = "https://" + *apiURL
	}
	slack := newSetter(*apiToken, *apiURL)

	expiry := int64(0)
	if *expireStatus {
		expiry = 30
	}

	conn, err := mpd.Dial("tcp", *mpdAddress)
	if err != nil {
		log.Fatal(fmt.Errorf("Failed to connect to mpd: %w", err))
	}
	defer conn.Close()

	go func() {
		for range time.Tick(30 * time.Second) {
			conn.Ping()
		}
	}()

	watcher, err := mpd.NewWatcher("tcp", *mpdAddress, *mpdPassword, "player")
	if err != nil {
		log.Fatal(fmt.Errorf("Failed to create mpd watcher: %w", err))
	}
	defer watcher.Close()

	// Main loop
	var lastTitle string
	for range watcher.Event {
		attrs, err := conn.Status()
		if err != nil {
			err = setStatus(slack, *defaultEmoji, *defaultText, expiry)
		} else if attrs["state"] == "play" {
			song, err := conn.CurrentSong()
			if err == nil {
				title := song["Title"] + " - " + song["Artist"]
				if title != lastTitle {
					lastTitle = title
					if expiry > 0 {
						duration, err := strconv.ParseFloat(song["duration"], 64)
						if err != nil {
							fmt.Println("Failed to parse song duration:", err)
						} else {
							expiry += time.Now().Unix() + int64(duration)
						}
					}
					err = setStatus(slack, ":headphones:", title, expiry)
				}
			}
		}
		if err != nil {
			log.Println("Error:", err)
		}
	}
}
