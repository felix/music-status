package mpd

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	gompd "github.com/fhs/gompd/v2/mpd"

	"src.userspace.com.au/felix/mstatus"
)

type Client struct {
	addr     string
	conn     *gompd.Client
	password string

	events chan mstatus.Status

	startWatcher chan bool
	//chWatcherStop  chan bool

	log  mstatus.Logger
	done chan struct{}
}

func New(opts ...Option) (*Client, error) {
	out := &Client{
		addr:   "localhost:6600",
		events: make(chan mstatus.Status),

		startWatcher: make(chan bool),
		log:          func(...interface{}) {},
	}
	for _, o := range opts {
		if err := o(out); err != nil {
			return nil, err
		}
	}
	return out, nil
}

var errConnection = errors.New("no connection")

type Option option
type option func(*Client) error

func Addr(addr string) Option {
	return func(c *Client) error {
		c.addr = addr
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

func (c *Client) Events() chan mstatus.Status {
	return c.events
}

func (c *Client) Stop() error {
	close(c.done)
	c.conn.Close()
	return nil
}

func (c *Client) connect() {
	ticker := time.NewTicker(time.Second * 5)
	for {
		select {
		case <-c.done:
			return
		case <-ticker.C:
			if err := c.doConnect(); err != nil {
				errorf("failed to connect\n")
			} else {
				c.log("mpd connected", c.addr)
			}
		}
	}
}

func (c *Client) doConnect() error {
	if c.conn != nil {
		if err := c.conn.Ping(); err == nil {
			return nil
		}
		c.conn.Close()
		c.conn = nil
		c.log("disconnected from mpd retrying")
	}

	var err error
	c.conn, err = gompd.Dial("tcp", c.addr)
	if err != nil {
		return errConnection
	}
	c.log("connected to mpd")
	go func() { c.startWatcher <- true }()
	return nil
}

func (c *Client) Watch() error {
	c.log("mpd starting")

	c.done = make(chan struct{})

	c.doConnect()
	go c.connect()

	ticker := time.NewTicker(time.Second)

	status := mstatus.Status{
		Player: mstatus.Player{Name: "mpd"},
	}

	var mpdEvents chan string

	for {
		status.State = mstatus.StateStopped
		var err error
		select {
		case <-c.done:
			return nil

		case <-ticker.C:
			status.Track, err = c.fetchSong()
			if err == nil && status.Track != nil {
				status.State = mstatus.StatePlaying
			}
			c.events <- status

		case <-c.startWatcher:
			watcher, err := gompd.NewWatcher("tcp", c.addr, c.password, "player")
			if err != nil {
				errorf("Failed to watch MPD", err)
				time.AfterFunc(3*time.Second, func() {
					c.startWatcher <- true
				})
			}
			if err != nil {
				return err
			}
			mpdEvents = watcher.Event

		case <-mpdEvents:
			attrs, err := c.conn.Status()
			if err == nil {
				switch attrs["state"] {
				case "stop":
					status.State = mstatus.StateStopped
				case "paused":
					status.State = mstatus.StatePaused
				case "play":
					status.State = mstatus.StatePlaying
				}
			}
		}
		if status.State == mstatus.StateError {
			c.log("mpd error", err)
		}
	}
}

func (c *Client) fetchSong() (*mstatus.Track, error) {
	if c.conn == nil {
		return nil, errConnection
	}
	stat, err := c.conn.Status()
	if err != nil {
		return nil, err
	}
	if stat["state"] != "play" {
		return nil, nil
	}
	song, err := c.conn.CurrentSong()
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
	//c.log("mpd song", stat)

	parts := strings.SplitN(stat["time"], ":", 2)
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
	return &mstatus.Track{
		ID:          song["Id"],
		Title:       song["Title"],
		Artist:      song["Artist"],
		Album:       song["Album"],
		Duration:    duration,
		Elapsed:     elapsed,
		MbTrackID:   song["MUSICBRAINZ_TRACKID"], // recording ID
		MbReleaseID: song["MUSICBRAINZ_ALBUMID"], // album ID
		MbArtistID:  song["MUSICBRAINZ_ARTISTID"],
		// MUSICBRAINZ_WORKID:4a484ba1-22d4-4fd1-a29a-b1c49d1e5161
	}, nil
}

func errorf(format string, v ...interface{}) {
	fmt.Fprintf(os.Stderr, "mpd error: "+format, v...)
}
