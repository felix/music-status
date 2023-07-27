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

func TestListenbrainzHandle(t *testing.T) {
	playingTrack := mstatus.Track{
		ID:     "id",
		Title:  "title",
		Artist: "artist",
		Album:  "album",
	}
	addInfo := additionalInfo{
		SubmissionClient:        "music-status https://github.com/felix/music-status",
		SubmissionClientVersion: "0.1.0",
		ArtistMBIDS:             []string{""},
	}
	tests := map[string]struct {
		status  mstatus.Status
		current *payload
		sub     submission
		failure bool
	}{
		"stopped": {
			status: mstatus.Status{
				State: mstatus.StateStopped,
				Track: &playingTrack,
			},
			sub: submission{},
		},
		"new play": {
			status: mstatus.Status{
				State: mstatus.StatePlaying,
				Track: &playingTrack,
			},
			//current: &payload{Track: track{ID: "current"}},
			sub: submission{
				ListenType: "playing_now",
				Payloads: []payload{{
					Track: track{
						Title:          "title",
						Artist:         "artist",
						Album:          "album",
						AdditionalInfo: addInfo,
					}},
				},
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
			sub: submission{
				ListenType: "playing_now",
				Payloads: []payload{{
					Track: track{
						Title:          "continued play",
						Artist:         "artist",
						Album:          "album",
						AdditionalInfo: addInfo,
					}},
				},
			},
		},
		"old play": {
			status: mstatus.Status{
				State: mstatus.StatePlaying,
				Track: &mstatus.Track{
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
				t.Logf("submission: %#v", sub)
				fmt.Fprintln(w, "OK")
			}))
			defer ts.Close()

			c := &Client{
				token:  "token",
				apiURL: ts.URL,
				log:    mstatus.Logger(t.Log),
			}
			c.httpClient = ts.Client()
			c.apiURL = ts.URL
			c.current = tt.current

			ch := make(chan mstatus.Status)
			go func() {
				ch <- tt.status
				close(ch)
			}()
			c.Start(ch)
			if len(sub.Payloads) != len(tt.sub.Payloads) {
				t.Fatalf("got %d, want %d", len(sub.Payloads), len(tt.sub.Payloads))
			}
			if len(sub.Payloads) != 0 && len(tt.sub.Payloads) != 0 {
				if !reflect.DeepEqual(sub.Payloads[0].Track, tt.sub.Payloads[0].Track) {
					t.Fatalf("\ngot  %#v\nwant %#v", sub.Payloads[0].Track, tt.sub.Payloads[0].Track)
				}
			}

		})
	}

}
