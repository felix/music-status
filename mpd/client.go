package mpd

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	gompd "github.com/fhs/gompd/v2/mpd"

	"src.userspace.com.au/felix/mstatus"
)

type Client struct {
	host     string
	port     int
	password string
	log      mstatus.Logger
	done     chan struct{}
}

func New(opts ...Option) (*Client, error) {
	out := &Client{
		host: "localhost",
		port: 6600,
		log:  func(...interface{}) {},
	}
	for _, o := range opts {
		if err := o(out); err != nil {
			return nil, err
		}
	}
	return out, nil
}

type Option option
type option func(*Client) error

func Host(h string) Option {
	return func(c *Client) error {
		c.host = h
		return nil
	}
}

func Port(p int) Option {
	return func(c *Client) error {
		c.port = p
		return nil
	}
}
func Password(s string) Option {
	return func(c *Client) error {
		c.password = s
		return nil
	}
}
func Logger(l mstatus.Logger) Option {
	return func(c *Client) error {
		c.log = l
		return nil
	}
}

func (c *Client) Stop() error {
	close(c.done)
	return nil
}

func (c *Client) Watch(handlers []mstatus.Handler) error {
	addr := fmt.Sprintf("%s:%d", c.host, c.port)
	conn, err := gompd.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to connect to mpd: %w", err)
	}
	defer conn.Close()
	c.log("mpd connected", addr)

	c.done = make(chan struct{})

	go func() {
		for range time.Tick(30 * time.Second) {
			conn.Ping()
		}
	}()

	watcher, err := gompd.NewWatcher("tcp", addr, c.password, "player")
	if err != nil {
		return fmt.Errorf("failed to create mpd watcher: %w", err)
	}
	defer watcher.Close()

	ticker := time.NewTicker(2 * time.Second)

	var currentSong *mstatus.Song
	currentState := mstatus.StateStopped

	// Publish on start
	// if track, err := c.publishSong(handlers, mstatus.EventPlay, conn); err == nil {
	// 	currentEvent = mstatus.EventPlay
	// 	currentSong = mstatus.Status{Track: *track}
	// }

	for {
		select {
		case <-c.done:
			return nil

		case <-ticker.C:
			currentSong, err = c.fetchSong(conn)
			if err != nil {
				c.log("mpd error", err)
				continue
			}
			if currentSong != nil {
				//c.log("mpd tick", currentEvent, currentSong, currentSong.Elapsed)
				currentState = mstatus.StatePlaying
				err := publishToAll(handlers, currentState, mstatus.Status{
					Player: mstatus.Player{Name: "mpd"},
					Track:  *currentSong})
				if err != nil {
					c.log("mpd publish error", err)
				}
			}

		case <-watcher.Event:
			attrs, err := conn.Status()
			if err != nil {
				currentState = mstatus.StateError
				currentSong = nil
				continue
			}

			c.log("mpd state", attrs["state"])
			switch attrs["state"] {
			case "stop":
				currentState = mstatus.StateStopped
			case "paused":
				currentState = mstatus.StatePaused
			case "play":
				currentState = mstatus.StatePlaying
			}
		}
	}
}

func (c *Client) fetchSong(conn *gompd.Client) (*mstatus.Song, error) {
	status, err := conn.Status()
	if err != nil {
		return nil, err
	}
	if status["state"] != "play" {
		return nil, nil
	}
	song, err := conn.CurrentSong()
	if err != nil {
		return nil, err
	}
	// Album:461 Ocean Boulevard
	// AlbumArtist:Eric Clapton
	// AlbumArtistSort:Clapton, Eric
	// Artist:Eric Clapton
	// ArtistSort:Clapton, Eric
	// Date:1974
	// Disc:1
	// Format:44100:16:2
	// Id:178
	// Label:RSO
	// Last-Modified:2022-02-21T06:20:53Z
	// MUSICBRAINZ_ALBUMARTISTID:618b6900-0618-4f1e-b835-bccb17f84294
	// MUSICBRAINZ_ALBUMID:2089dcff-a209-49c4-8bbe-d43328c6efed
	// MUSICBRAINZ_ARTISTID:618b6900-0618-4f1e-b835-bccb17f84294
	// MUSICBRAINZ_RELEASETRACKID:8516da10-ebe3-47c4-b33b-501e6250cbce
	// MUSICBRAINZ_TRACKID:10aae51f-f253-42c4-8af8-5673da1c98e6
	// MUSICBRAINZ_WORKID:4a484ba1-22d4-4fd1-a29a-b1c49d1e5161
	// OriginalDate:1974
	// Pos:24
	// Time:292
	// Title:Motherless
	// Children
	// Track:1
	// duration:291.549
	// file:Eric_Clapton/461_Ocean_Boulevard/01_Motherless_Children.flac
	//c.log("mpd song", status)

	parts := strings.SplitN(status["time"], ":", 2)
	var duration time.Duration
	secs, err := strconv.ParseFloat(parts[1], 64)
	if err == nil {
		duration = time.Duration(secs * float64(time.Second))
	}
	var elapsed time.Duration
	secs, err = strconv.ParseFloat(parts[0], 64)
	if err == nil {
		elapsed = time.Duration(secs * float64(time.Second))
	}
	track := mstatus.Song{
		ID:          song["Id"],
		Title:       song["Title"],
		Artist:      song["Artist"],
		Album:       song["Album"],
		Duration:    duration,
		Elapsed:     elapsed,
		MbTrackID:   song["MUSICBRAINZ_TRACKID"],
		MbReleaseID: song["MUSICBRAINZ_ALBUMID"],
		MbArtistID:  song["MUSICBRAINZ_ARTISTID"],
		// MUSICBRAINZ_WORKID:4a484ba1-22d4-4fd1-a29a-b1c49d1e5161
	}
	//c.log("mpd set song", track)
	return &track, nil
}

func publishToAll(handlers []mstatus.Handler, e mstatus.State, s mstatus.Status) error {
	for _, p := range handlers {
		if err := p.Handle(e, s); err != nil {
			return fmt.Errorf("publish error: %w", err)
		}
	}
	return nil
}
