package lastfm

import (
	"errors"
	"time"

	"github.com/shkh/lastfm-go/lastfm"

	"src.userspace.com.au/felix/mstatus"
)

const (
	scope = "lastfm"
)

type Client struct {
	api      *lastfm.Api
	username string

	events chan mstatus.Status

	startWatcher chan bool
	//chWatcherStop  chan bool

	log  mstatus.Logger
	done chan struct{}
}

func init() {
	mstatus.Register(&Client{
		events: make(chan mstatus.Status),

		startWatcher: make(chan bool),
		log:          func(...interface{}) {},
	})
}

func (c *Client) Name() string {
	return scope
}

func (c *Client) Load(cfg mstatus.Config, log mstatus.Logger) error {
	c.log = log
	key := cfg.ReadString(scope, "key")
	c.username = cfg.ReadString(scope, "username")
	c.api = lastfm.New(key, "")
	return nil
}

var errConnection = errors.New("no connection")

func (c *Client) Events() chan mstatus.Status {
	return c.events
}

func (c *Client) Stop() error {
	close(c.done)
	return nil
}

func (c *Client) Watch() error {
	c.log("lastfm starting")

	c.done = make(chan struct{})

	ticker := time.NewTicker(3 * time.Second)

	status := mstatus.Status{
		Player: mstatus.Player{Name: scope},
	}

	for {
		status.State = mstatus.StateStopped

		select {
		case <-c.done:
			return nil

		case <-ticker.C:
			result, err := c.api.User.GetRecentTracks(
				lastfm.P{
					"limit": "1",
					"user":  c.username,
				},
			)
			if err != nil {
				c.log("failed to get recent tracks", err)
				//return err
			}

			if len(result.Tracks) == 0 {
				c.events <- status
				continue
			}

			ctrack := result.Tracks[0]
			//isNowPlaying, err := strconv.ParseBool(ctrack.NowPlaying)
			//if err != nil && ctrack.NowPlaying != "" {
			//	c.log("failed to parse", err)
			//	//return err
			//}

			// if !isNowPlaying {
			// 	c.events <- status
			// 	continue
			// }

			status.Track = &mstatus.Track{
				ID:     ctrack.Mbid,
				Title:  ctrack.Name,
				Artist: ctrack.Artist.Name,
				Album:  ctrack.Album.Name,
				//Duration:    ,
				//Elapsed:     elapsed,
				MbTrackID:   ctrack.Mbid,
				MbReleaseID: ctrack.Artist.Mbid,
				MbArtistID:  ctrack.Artist.Mbid,
			}
			status.State = mstatus.StatePlaying
			c.events <- status
		}
	}
}
