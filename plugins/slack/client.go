package slack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"src.userspace.com.au/felix/mstatus"
)

func init() {
	mstatus.Register(&Client{
		apiURL:     defaultURL,
		httpClient: &http.Client{},
		expiry:     time.Duration(5 * time.Minute),
		emoji:      defaultEmoji,
		log:        func(...interface{}) {},
	})
}

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
	scope        = "slack"
	defaultURL   = "https://api.slack.com"
	slackAction  = "users.profile.set"
	defaultEmoji = ":musical_note:"
)

func (c *Client) Name() string {
	return scope
}

func (c *Client) Load(sess *mstatus.Session, log mstatus.Logger) error {
	c.log = log
	if s := sess.ConfigString(scope, "token"); s != "" {
		c.token = s
	}
	if s := sess.ConfigString(scope, "url"); s != "" {
		if !strings.HasPrefix(s, "http") {
			s = "https://" + s
		}
		c.apiURL = s
	}
	if s := sess.ConfigString(scope, "expireStatus"); s != "" {
		d, err := time.ParseDuration(s)
		if err != nil {
			return err
		}
		c.expiry = d
	}
	if s := sess.ConfigString(scope, "defaultEmoji"); s != "" {
		c.defaultEmoji = s
	}
	if s := sess.ConfigString(scope, "defaultStatus"); s != "" {
		c.defaultStatus = s
	}
	if s := sess.ConfigString(scope, "emoji"); s != "" {
		c.emoji = s
	}
	if c.token == "" {
		return fmt.Errorf("missing slack token")
	}

	return nil
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

func (c *Client) Stop() error {
	return c.setStatus(payload{
		StatusText:       c.defaultStatus,
		StatusEmoji:      c.defaultEmoji,
		StatusExpiration: 0,
	})
}

func (c *Client) setStatus(p payload) error {
	uri, err := url.JoinPath(c.apiURL, slackAction)
	if err != nil {
		return err
	}
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
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.log("failed to read body", err)
	}
	if err := json.Unmarshal(body, &r); err != nil {
		c.log("slack invalid response", err, string(body))
	}
	if !r.OK {
		c.log("slack failure", r.Warning, r.Error)
	}
	return nil
}

func errorf(format string, v ...interface{}) {
	fmt.Fprintf(os.Stderr, "slack error: "+format, v...)
}
