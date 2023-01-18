package qobuz

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"src.userspace.com.au/felix/mstatus"
)

type Client struct {
	appID  string
	secret string
	token  string

	httpClient doer

	events chan mstatus.Status
	log    mstatus.Logger
}

type doer interface {
	Do(req *http.Request) (*http.Response, error)
}

func New(user, pass string, opts ...Option) (*Client, error) {
	out := &Client{
		events: make(chan mstatus.Status),
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

const (
	playBaseURL = "https://play.qobuz.com"
	apiBaseURL  = "https://www.qobuz.com/api.json/0.2/"
)

type Option option
type option func(*Client) error

func AppID(s string) Option {
	return func(c *Client) error {
		c.appID = s
		return nil
	}
}

func AppSecret(s string) Option {
	return func(c *Client) error {
		c.secret = s
		return nil
	}
}

func Logger(l mstatus.Logger) Option {
	return func(c *Client) error {
		c.log = l
		return nil
	}
}

var (
	tzRE      = regexp.MustCompile(`[a-z]\.initialSeed\("([\w=]+)",window\.utimezone\.([a-z]+)\)`)
	extrasREs = `name:"\w+/(__TIMEZONES__)",info:"([\w=]+)",extras:"([\w=]+)"`
	//appIDRE  = regexp.MustCompile(`app_id:"(\d+)",app_secret:"\w+"`)
	appIDRE  = regexp.MustCompile(`{app_id:"(\d{9})",app_secret:"\w{32}",base_port:"80",base_url:"https://www\.qobuz\.com",base_method:"/api\.json/0\.2/"},n`)
	bundleRE = regexp.MustCompile(`src="(/resources/\d+\.\d+\.\d+-[a-z]\d+/bundle\.js)"`)
)

func (c *Client) getAppSecret() error {
	// Get bundle
	resp, err := http.Get(playBaseURL + "/login")
	if err != nil {
		return err
	}
	loginContent, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	bundleURL := bundleRE.FindSubmatch(loginContent)
	if bundleURL == nil {
		return fmt.Errorf("bundle URL not found")
	}
	resp, err = http.Get(playBaseURL + string(bundleURL[1]))
	if err != nil {
		return err
	}
	bundleContent, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Get AppID
	appID := appIDRE.FindSubmatch(bundleContent)
	if appID == nil {
		return fmt.Errorf("appID not found")
	}
	c.appID = string(appID[1])

	seeds := tzRE.FindAllSubmatch(bundleContent, -1)
	if seeds == nil {
		return fmt.Errorf("seed not found")
	}

	type secret struct {
		Zone string
		Data []byte
	}
	var secrets []secret
	var timezones []string
	for _, seed := range seeds {
		zone := strings.ToLower(string(seed[2]))
		tcZone := cases.Title(language.English).String(zone)
		secrets = append(secrets, secret{Zone: zone, Data: seed[1]})
		timezones = append(timezones, tcZone)
	}

	// Create secret
	extrasRE := regexp.MustCompile(strings.Replace(extrasREs, "__TIMEZONES__", strings.Join(timezones, "|"), 1))
	matches := extrasRE.FindAllSubmatch(bundleContent, -1)
	if matches == nil {
		return fmt.Errorf("secrets not found")
	}

	for _, m := range matches {
		lZone := strings.ToLower(string(m[1]))
		for i, s := range secrets {
			if s.Zone == lZone {
				secrets[i].Data = append(secrets[i].Data, m[2]...)
				secrets[i].Data = append(secrets[i].Data, m[3]...)
				b64 := string(secrets[i].Data)
				sec, err := base64.StdEncoding.DecodeString(b64[:44])
				if err != nil {
					return err
				}
				// TODO is this secret always correct?
				c.secret = string(sec)
				return nil
			}
		}
	}
	return fmt.Errorf("unable to extract app secret")
}

func (c *Client) checkAuth() error {
	ts := time.Now().Unix()
	trackID := "5966783"
	fmtID := "5"
	sig := md5.Sum([]byte(fmt.Sprintf(
		"trackgetFileUrlformat_id%sintentstreamtrack_id%s%d%s",
		fmtID, trackID, ts, c.secret,
	)))
	params := &url.Values{
		"request_ts":  []string{strconv.FormatInt(ts, 10)},
		"request_sig": []string{hex.EncodeToString(sig[:])},
		"track_id":    []string{trackID},
		"format_id":   []string{fmtID},
		"intent":      []string{"stream"},
		"app_id":      []string{c.appID},
	}
	req, err := http.NewRequest(http.MethodPost, apiBaseURL+"track/getFileUrl", strings.NewReader(params.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_, err = c.callAPI(req)
	return err
}

func (c *Client) auth(u, p string) error {
	pHash := md5.Sum([]byte(p))
	params := &url.Values{
		"username": []string{u},
		"email":    []string{""},
		"password": []string{hex.EncodeToString(pHash[:])},
	}
	req, err := http.NewRequest(http.MethodPost, apiBaseURL+"user/login", strings.NewReader(params.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	body, err := c.callAPI(req)
	if err != nil {
		return err
	}
	var info struct {
		Token string `json:"user_auth_token"`
	}
	if err := json.Unmarshal(body, &info); err != nil {
		return err
	}
	c.token = info.Token
	// self.label = usr_info["user"]["credential"]["parameters"]["short_label"]
	return nil
}

func (c *Client) callAPI(req *http.Request) ([]byte, error) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:108.0) Gecko/20100101 Firefox/108.0")
	req.Header.Set("X-App-Id", c.appID)
	if c.token != "" {
		req.Header.Set("X-User-Auth-Token", c.token)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode > 299 {
		err = fmt.Errorf("error response for %q: %d", req.URL.Path, resp.StatusCode)
	}
	return out, err
}

func (c *Client) Watch() error {
	return nil
}
func (c *Client) Events() chan mstatus.Status {
	return nil
}
func (c *Client) Stop() error {
	return nil
}
