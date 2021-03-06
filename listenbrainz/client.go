package listenbrainz

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"src.userspace.com.au/felix/mstatus"
)

const defaultURL = "https://api.listenbrainz.org/1/submit-listens"

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

	Title          string         `json:"track_name"`
	Artist         string         `json:"artist_name"`
	Album          string         `json:"release_name"`
	AdditionalInfo additionalInfo `json:"additional_info,omitempty"`
}

type additionalInfo struct {
	MediaPlayer             string   `json:"media_player,omitempty"`
	SubmissionClient        string   `json:"submission_client,omitempty"`
	SubmissionClientVersion string   `json:"submission_client_version,omitempty"`
	ReleaseMBID             string   `json:"release_mbid,omitempty"`
	ArtistMBIDS             []string `json:"artist_mbids,omitempty"`
	RecordingMBID           string   `json:"recording_mbid,omitempty"`
	WorkMBIDs               []string `json:"work_mbids,omitempty"`
	Tags                    []string `json:"tags,omitempty"`
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

func (c *Client) Start(events <-chan mstatus.Status) {
	for event := range events {
		switch event.State {
		case mstatus.StatePlaying:
			if c.current != nil {
				newTrack := c.current.Track.id != event.Track.ID
				if newTrack || !c.current.Track.singleSent {
					// Listens should be submitted for tracks when the user has listened to
					// half the track or 4 minutes of the track, whichever is lower. If the
					// user hasn???t listened to 4 minutes or half the track, it doesn???t
					// fully count as a listen and should not be submitted.
					// https://listenbrainz.readthedocs.io/en/latest/users/api/core/#post--1-submit-listens
					elapsed := event.Track.Elapsed.Seconds()
					pc := elapsed / event.Track.Duration.Seconds()
					if pc > 0.5 || (elapsed > 240) {
						if err := c.submit(submission{
							ListenType: "single",
							Payloads:   []payload{*c.current},
						}); err != nil {
							errorf("failed to submit: %s", err)
						}
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
						id:     event.Track.ID,
						Title:  event.Track.Title,
						Artist: event.Track.Artist,
						Album:  event.Track.Album,
						AdditionalInfo: additionalInfo{
							// TODO store this somewhere else
							MediaPlayer:             event.Player.Name,
							SubmissionClient:        "music-status https://github.com/felix/music-status",
							SubmissionClientVersion: "0.1.0",
							ReleaseMBID:             event.Track.MbReleaseID,
							ArtistMBIDS:             []string{event.Track.MbArtistID},
							//RecordingMBID: string
							//Tags: []string
						},
					},
				}
			}

			if !c.current.Track.playingSent {
				if err := c.submit(submission{
					ListenType: "playing_now",
					Payloads:   []payload{*c.current},
				}); err != nil {
					errorf("failed to submit: %s", err)
				}
				c.current.ListenedAt = time.Now().Unix()
				c.current.Track.playingSent = true
			}

		case mstatus.StateStopped:
			c.current = nil
		}
	}
}

func (c *Client) submit(sub submission) error {
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	if err := enc.Encode(sub); err != nil {
		return err
	}
	req, err := http.NewRequest("POST", c.apiURL, buf)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Token %s", c.token))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		var body []byte
		if resp != nil && resp.StatusCode != 200 {
			if resp.Body != nil {
				body, _ = ioutil.ReadAll(resp.Body)
			}
			err = fmt.Errorf("failed to set status: %w %d %q", err, resp.StatusCode, string(body))
		}
		return err
	}
	c.log("listenbrainz published", sub.ListenType, sub.Payloads[0])
	return nil
}

func errorf(format string, v ...interface{}) {
	fmt.Fprintf(os.Stderr, "listenbrainz error: "+format, v...)
}
