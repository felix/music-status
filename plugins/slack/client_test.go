package slack

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"src.userspace.com.au/felix/mstatus"
)

func TestSlackHandle(t *testing.T) {
	playingTrack := mstatus.Track{
		ID:     "id",
		Title:  "title",
		Artist: "artist",
		Album:  "album",
	}
	tests := map[string]struct {
		status  mstatus.Status
		pl      payload
		failure bool
	}{
		"stopped": {
			status: mstatus.Status{
				State: mstatus.StateStopped,
				Track: &playingTrack,
			},
			pl: payload{},
		},
		"new play": {
			status: mstatus.Status{
				State: mstatus.StatePlaying,
				Track: &playingTrack,
			},
			pl: payload{
				StatusText:       `"title" by artist`,
				StatusEmoji:      defaultEmoji,
				StatusExpiration: 0,
			},
		},
		"continued play": {
			status: mstatus.Status{
				State: mstatus.StatePlaying,
				Track: &mstatus.Track{
					ID:       "id",
					Title:    "continued play",
					Artist:   "artist",
					Album:    "album",
					Duration: 90 * time.Second,
					Elapsed:  time.Minute,
				}},
			pl: payload{
				StatusText:       `"continued play" by artist`,
				StatusEmoji:      defaultEmoji,
				StatusExpiration: 0,
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var pl map[string]payload
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				dec := json.NewDecoder(r.Body)
				defer r.Body.Close()
				if err := dec.Decode(&pl); err != nil {
					t.Fatalf("failed to decode %s", err)
				}
				t.Logf("payload: %#v", pl)
				fmt.Fprintln(w, `{"ok":true}`)
			}))
			defer ts.Close()

			c := &Client{
				token:  "token",
				apiURL: ts.URL,
				log:    mstatus.Logger(t.Log),
			}
			c.httpClient = ts.Client()

			ch := make(chan mstatus.Status)
			go func() {
				ch <- tt.status
				close(ch)
			}()
			c.Start(ch)
			sub, ok := pl["profile"]
			if ok {
				if sub.StatusText != tt.pl.StatusText {
					t.Fatalf("got %q, want %q", sub.StatusText, tt.pl.StatusText)
				}
				if sub.StatusEmoji != tt.pl.StatusEmoji {
					t.Fatalf("got %q, want %q", sub.StatusEmoji, tt.pl.StatusEmoji)
				}
			}
		})
	}

}
