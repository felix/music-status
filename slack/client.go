package slack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"src.userspace.com.au/felix/mstatus"
)

type Client struct {
	token      string
	apiURL     string
	httpClient *http.Client
	log        mstatus.Logger

	// Expiry this number of seconds after the song finishes
	expiry time.Duration
	// Emoji to use when unpublishing
	defaultEmoji string
	// Status to use when unpublishing
	defaultStatus string
	// Emoji to use for publishing
	emoji string
}

// payload is the structure sent to Slack
type payload struct {
	StatusText       string `json:"status_text"`
	StatusEmoji      string `json:"status_emoji"`
	StatusExpiration int64  `json:"status_expiration"`
}

func (p payload) String() string {
	return p.StatusText
}

const (
	slackAction  = "/users.profile.set"
	defaultEmoji = ":musical_note:"
)

func New(token, url string, opts ...Option) (*Client, error) {
	if !strings.HasPrefix(url, "http") {
		url = "https://" + url
	}
	out := &Client{
		token:      token,
		apiURL:     url,
		httpClient: &http.Client{},
		expiry:     time.Duration(15 * time.Second),
		emoji:      defaultEmoji,
		log:        func(...interface{}) {},
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

func ExpireAfter(s string) Option {
	return func(c *Client) error {
		if s == "" {
			return nil
		}
		d, err := time.ParseDuration(s)
		c.expiry = d
		return err
	}
}

func DefaultEmoji(e string) Option {
	return func(c *Client) error {
		if e == "" {
			return nil
		}
		c.defaultEmoji = e
		return nil
	}
}

func DefaultStatus(s string) Option {
	return func(c *Client) error {
		if s == "" {
			return nil
		}
		c.defaultStatus = s
		return nil
	}
}

func Emoji(e string) Option {
	return func(c *Client) error {
		if e == "" {
			return nil
		}
		c.emoji = e
		return nil
	}
}
func Logger(l mstatus.Logger) Option {
	return func(c *Client) error {
		c.log = l
		return nil
	}
}

func (c *Client) Start(events <-chan mstatus.Status) {
	var lastStatus string
	for event := range events {
		switch event.State {
		case mstatus.StateError, mstatus.StateStopped, mstatus.StatePaused:
			if lastStatus == "" {
				continue
			}
			lastStatus = ""
			if err := c.setStatus(payload{
				StatusText:       c.defaultStatus,
				StatusEmoji:      c.defaultEmoji,
				StatusExpiration: 0,
			}); err != nil {
				errorf("failed to set status: %s", err)
			}

		case mstatus.StatePlaying:
			s := event.Track
			if lastStatus == s.String() {
				continue
			}

			var expiry int64
			if c.expiry > 0 {
				expiry += time.Now().Add(c.expiry).Add(s.Duration).Unix()
			}

			lastStatus = s.String()
			if err := c.setStatus(payload{
				StatusText:       lastStatus,
				StatusEmoji:      c.emoji,
				StatusExpiration: expiry,
			}); err != nil {
				errorf("failed to set status: %s", err)
			}
		default:
			c.log("slack unhandled state", event)
		}
	}
}

func (c *Client) setStatus(p payload) error {
	uri := c.apiURL + slackAction
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	if err := enc.Encode(map[string]payload{"profile": p}); err != nil {
		return err
	}
	req, err := http.NewRequest("POST", uri, buf)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if p.StatusText != "" {
		c.log("slack published", p)
	}

	var r = struct {
		OK      bool   `json:"ok"`
		Warning string `json:"warning"`
		Error   string `json:"error"`
	}{}
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&r); err != nil {
		c.log("slack invalid response", err)
	}
	if !r.OK {
		c.log("slack failure", r.Warning, r.Error)
	}
	return nil
}

func errorf(format string, v ...interface{}) {
	fmt.Fprintf(os.Stderr, "slack error: "+format, v...)
}
