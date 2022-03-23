package mpd

import (
	"fmt"
	"strconv"
	"time"

	gompd "github.com/fhs/gompd/mpd"

	"src.userspace.com.au/felix/mstatus"
)

type Client struct {
	host     string
	port     int
	password string
	conn     *gompd.Client
	log      mstatus.Logger
}

func New(opts ...Option) (*Client, error) {
	out := &Client{
		host: "localhost",
		port: 6600,
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

func (c *Client) Watch(pubs []mstatus.Publisher) error {
	addr := fmt.Sprintf("%s:%d", c.host, c.port)
	conn, err := gompd.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to connect to mpd: %w", err)
	}
	defer conn.Close()
	c.log("mpd connected", addr)

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

	for range watcher.Event {
		attrs, err := conn.Status()
		if err != nil {
			err = publish(pubs, mstatus.Status{
				Error: fmt.Errorf("mpd invalid status: %w", err),
			})
		} else if attrs["state"] == "play" {
			song, err := conn.CurrentSong()
			if err != nil {
				c.log("mpd song error", err)
			} else {
				var duration time.Duration
				secs, err := strconv.ParseFloat(song["duration"], 64)
				if err == nil {
					duration = time.Duration(secs * float64(time.Second))
				}
				track := mstatus.Song{
					Title:    song["Title"],
					Artist:   song["Artist"],
					Duration: duration,
				}
				c.log("mpd status change", track.String())
				err = publish(pubs, mstatus.Status{Track: track})
				if err != nil {
					c.log("mpd publish error", err)
				}
			}
		}
	}
	return nil
}

func publish(pubs []mstatus.Publisher, s mstatus.Status) error {
	for _, p := range pubs {
		if err := p.Publish(s); err != nil {
			return fmt.Errorf("publish error: %w", err)
		}
	}
	return nil
}
