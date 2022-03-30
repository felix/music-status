package listenbrainz

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"src.userspace.com.au/felix/mstatus"
)

func TestHandle(t *testing.T) {
	playingTrack := mstatus.Song{
		ID:     "id",
		Artist: "artist",
		Album:  "album",
	}
	tests := map[string]struct {
		state   mstatus.State
		status  mstatus.Status
		current *payload
		sub     submission
		failure bool
	}{
		"stopped": {
			state:  mstatus.StateStopped,
			status: mstatus.Status{Track: playingTrack},
			sub:    submission{},
		},
		"new play": {
			state:  mstatus.StatePlaying,
			status: mstatus.Status{Track: playingTrack},
			//current: &payload{Track: track{ID: "current"}},
			sub: submission{
				ListenType: "playing_now",
				Payloads: []payload{{
					Track: track{
						Artist: "artist",
						Album:  "album",
					}},
				},
			},
		},
		"continued play": {
			state: mstatus.StatePlaying,
			status: mstatus.Status{Track: mstatus.Song{
				ID:       "id",
				Artist:   "artist",
				Album:    "album",
				Duration: 90 * time.Second,
				Elapsed:  time.Minute,
			}},
			sub: submission{
				ListenType: "playing_now",
				Payloads: []payload{{
					Track: track{
						Artist: "artist", Album: "album",
					}},
				},
			},
		},
		"old play": {
			state: mstatus.StatePlaying,
			status: mstatus.Status{Track: mstatus.Song{
				ID:       "id",
				Artist:   "artist",
				Album:    "album",
				Duration: 90 * time.Second,
				Elapsed:  time.Minute,
			}},
			current: &payload{Track: track{
				id:          "id",
				singleSent:  true,
				playingSent: true,
			}},
			sub: submission{},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var sub submission
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				dec := json.NewDecoder(r.Body)
				if err := dec.Decode(&sub); err != nil {
					t.Fatalf("failed to decode %s", err)
				}
				fmt.Fprintln(w, "OK")
			}))
			defer ts.Close()

			c, _ := New("token")
			c.httpClient = ts.Client()
			c.apiURL = ts.URL
			c.current = tt.current

			err := c.Handle(tt.state, tt.status)
			if err != nil && !tt.failure {
				t.Fatalf("got %s, want nil", err)
			}
			if err == nil && tt.failure {
				t.Fatalf("got nil, want failure")
			}
			if !reflect.DeepEqual(sub, tt.sub) {
				t.Fatalf("got %#v, want %#v", sub, tt.sub)
			}

		})
	}

}
