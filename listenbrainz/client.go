package listenbrainz

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"src.userspace.com.au/felix/mstatus"
)

const defaultURL = "https://api.listenbrainz.org"

type Client struct {
	token      string
	apiURL     string
	httpClient *http.Client
	log        mstatus.Logger

	current *payload
}

type submission struct {
	ListenType string    `json:"listen_type"`
	Payloads   []payload `json:"payload"`
}

type payload struct {
	ListenedAt int64 `json:"listened_at,omitempty"`
	Track      track `json:"track_metadata"`
}

func (p payload) String() string {
	return fmt.Sprintf("%q by %s", p.Track.Title, p.Track.Artist)
}

// Track is a helper struct for marshalling the JSON payload
type track struct {
	id          string
	singleSent  bool
	playingSent bool

	Title  string `json:"track_name"`
	Artist string `json:"artist_name"`
	Album  string `json:"release_name"`
}

func New(token string, opts ...Option) (*Client, error) {
	out := &Client{
		token:  token,
		apiURL: defaultURL,
		log:    func(...interface{}) {},
	}
	for _, o := range opts {
		if err := o(out); err != nil {
			return nil, err
		}
	}
	if out.httpClient == nil {
		out.httpClient = &http.Client{}
	}
	return out, nil
}

type Option option

type option func(*Client) error

func Logger(l mstatus.Logger) Option {
	return func(c *Client) error {
		c.log = l
		return nil
	}
}

func (c *Client) Handle(event mstatus.State, status mstatus.Status) error {
	switch event {
	case mstatus.StatePlaying:
		if c.current != nil {
			newTrack := c.current.Track.id != status.Track.ID
			if newTrack || !c.current.Track.singleSent {
				// Listens should be submitted for tracks when the user has listened to
				// half the track or 4 minutes of the track, whichever is lower. If the
				// user hasn’t listened to 4 minutes or half the track, it doesn’t
				// fully count as a listen and should not be submitted.
				// https://listenbrainz.readthedocs.io/en/latest/users/api/core/#post--1-submit-listens
				elapsed := status.Track.Elapsed.Seconds()
				pc := elapsed / status.Track.Duration.Seconds()
				if pc > 0.5 || (elapsed > 240) {
					c.submit(submission{
						ListenType: "single",
						Payloads:   []payload{*c.current},
					})
					c.current.Track.singleSent = true
				}
				if newTrack {
					c.current = nil
				}
			}
		}
		if c.current == nil {
			c.current = &payload{
				Track: track{
					id:     status.Track.ID,
					Title:  status.Track.Title,
					Artist: status.Track.Artist,
					Album:  status.Track.Album,
					// TODO add musicbrainz
				},
			}
		}

		if !c.current.Track.playingSent {
			c.submit(submission{
				ListenType: "playing_now",
				Payloads:   []payload{*c.current},
			})
			c.current.ListenedAt = time.Now().Unix()
			c.current.Track.playingSent = true
		}

	case mstatus.StateStopped:
		c.current = nil
	}
	return nil
}

func (c *Client) submit(sub submission) error {
	uri := c.apiURL + "/1/submit-listens"
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	if err := enc.Encode(sub); err != nil {
		return err
	}
	req, err := http.NewRequest("POST", uri, buf)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Token %s", c.token))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		body, _ := ioutil.ReadAll(resp.Body)
		e := fmt.Sprintf("failed to set status: %s %d %q", err, resp.StatusCode, string(body))
		c.log("listenbrainz failure:", e)
		return fmt.Errorf(e)
	}
	c.log("listenbrainz published", sub.ListenType, sub.Payloads[0])
	return nil
}
