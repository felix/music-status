package spotify

import (
	"context"
	"fmt"
	"net/http"
	"time"

	spot "github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
	"golang.org/x/oauth2"
	"src.userspace.com.au/felix/mstatus"
)

const redirectURI = "http://localhost:8080/callback"

func init() {
	mstatus.Register(&Client{
		events: make(chan mstatus.Status),
		log:    func(...interface{}) {},
	})
}

type Client struct {
	events       chan mstatus.Status
	auth         *spotifyauth.Authenticator
	api          *spot.Client
	clientID     string
	clientSecret string
	log          mstatus.Logger
	done         chan struct{}
}

var _ mstatus.Source = (*Client)(nil)

const scope = "spotify"

func (c *Client) Name() string {
	return scope
}

func (c *Client) Load(cfg mstatus.Config, log mstatus.Logger) error {
	c.log = log
	c.clientID = cfg.ReadString(scope, "client_id")
	c.clientSecret = cfg.ReadString(scope, "client_secret")
	c.done = make(chan struct{})
	return nil
}

func (c *Client) Events() chan mstatus.Status {
	return c.events
}

func (c *Client) Stop() error {
	close(c.done)
	return nil
}

func (c *Client) doAuth(log mstatus.Logger) error {
	var ch = make(chan *spot.Client)
	var state = "abc123"

	spotifyauth.ShowDialog = oauth2.SetAuthURLParam("show_dialog", "false")
	c.auth = spotifyauth.New(
		spotifyauth.WithClientID(c.clientID),
		spotifyauth.WithClientSecret(c.clientSecret),
		spotifyauth.WithRedirectURL(redirectURI),
		spotifyauth.WithScopes(
			spotifyauth.ScopeUserReadCurrentlyPlaying,
			spotifyauth.ScopeUserReadPlaybackState,
		),
	)

	completeAuth := func(w http.ResponseWriter, r *http.Request) {
		tok, err := c.auth.Token(r.Context(), state, r)
		if err != nil {
			http.Error(w, "Couldn't get token", http.StatusForbidden)
			log(err)
		}
		if st := r.FormValue("state"); st != state {
			http.NotFound(w, r)
			log("State mismatch: %s != %s\n", st, state)
		}

		// use the token to get an authenticated client
		client := spot.New(c.auth.Client(r.Context(), tok))
		fmt.Fprintf(w, "Login Completed!")
		ch <- client
	}

	http.HandleFunc("/callback", completeAuth)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log("Got request for:", r.URL.String())
	})
	go func() {
		err := http.ListenAndServe(":8080", nil)
		if err != nil {
			log(err)
		}
	}()

	url := c.auth.AuthURL(state)
	fmt.Println("Please log in to Spotify by visiting the following page in your browser:", url)

	// wait for auth to complete
	c.api = <-ch
	return nil
}

func (c *Client) Watch() error {
	c.log("spotify starting")

	if err := c.doAuth(c.log); err != nil {
		return err
	}

	ticker := time.NewTicker(3 * time.Second)

	status := mstatus.Status{
		Player: mstatus.Player{Name: scope},
	}

	ctx := context.Background()

	for {
		status.State = mstatus.StateStopped

		select {
		case <-c.done:
			return nil

		case <-ticker.C:
			cTrack, err := c.api.PlayerCurrentlyPlaying(ctx)
			if err != nil {
				c.log("failed to get recent tracks", err)
				continue
			}

			if !cTrack.Playing {
				c.events <- status
				continue
			}

			artist := ""
			if len(cTrack.Item.Artists) > 0 {
				artist = cTrack.Item.Artists[0].Name
			}

			status.Track = &mstatus.Track{
				ID:       cTrack.Item.ID.String(),
				Title:    cTrack.Item.Name,
				Artist:   artist,
				Album:    cTrack.Item.Album.Name,
				Duration: cTrack.Item.TimeDuration(),
				//Elapsed:     elapsed,
			}
			status.State = mstatus.StatePlaying
			c.events <- status
		}
	}
}
