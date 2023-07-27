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
	sess         *mstatus.Session
	auth         *spotifyauth.Authenticator
	api          *spot.Client
	loginServer  *http.Server
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

func (c *Client) Load(sess *mstatus.Session, log mstatus.Logger) error {
	c.log = log
	c.clientID = sess.ConfigString(scope, "client_id")
	c.clientSecret = sess.ConfigString(scope, "client_secret")
	c.sess = sess
	c.done = make(chan struct{})
	return nil
}

func (c *Client) Events() chan mstatus.Status {
	return c.events
}

func (c *Client) Stop() error {
	close(c.done)
	if c.loginServer != nil {
		return c.loginServer.Close()
	}
	return nil
}

func (c *Client) Watch() error {
	c.log("spotify starting")

	var err error
	var token oauth2.Token
	if err := c.sess.ReadState(scope, &token); err != nil {
		return err
	}

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

	if token.AccessToken == "" {
		if token, err = c.getToken(c.log); err != nil {
			return err
		}
		if token.AccessToken != "" {
			if err := c.sess.WriteState(scope, token); err != nil {
				return err
			}
		}
	}

	c.api = spot.New(c.auth.Client(context.Background(), &token))

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

			if cTrack == nil || !cTrack.Playing {
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

func (c *Client) getToken(log mstatus.Logger) (oauth2.Token, error) {
	var ch = make(chan *oauth2.Token)
	var state = "abc123"

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
		fmt.Fprintf(w, "Login Completed!")
		ch <- tok
	}

	http.HandleFunc("/callback", completeAuth)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log("Got request for:", r.URL.String())
	})
	go func() {
		c.loginServer = &http.Server{Addr: ":8080", Handler: nil}
		if err := c.loginServer.ListenAndServe(); err != nil {
			log(err)
		}
	}()

	url := c.auth.AuthURL(state)
	fmt.Println("Please log in to Spotify by visiting the following page in your browser:", url)

	// wait for auth to complete
	tok := <-ch
	err := c.loginServer.Close()
	c.loginServer = nil
	return *tok, err
}
