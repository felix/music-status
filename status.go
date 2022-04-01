package mstatus

import (
	"fmt"
	"time"
)

type State string

const (
	// StateStopped is triggered when stopped
	StateStopped State = "stopped"
	// StatePlaying is triggered during playback
	StatePlaying State = "playing"
	// StatePaused is triggered when paused
	StatePaused State = "paused"
	// StateError is triggered on error
	StateError State = "error"
)

type Song struct {
	ID          string
	Title       string
	Artist      string
	Album       string
	Duration    time.Duration
	Elapsed     time.Duration
	MbArtistID  string
	MbTrackID   string
	MbReleaseID string
}

func (s Song) String() string {
	return fmt.Sprintf("(%s) %q by %s", s.ID, s.Title, s.Artist)

}

type Status struct {
	Player Player
	Track  Song
	Error  error
}

type Player struct {
	Name    string
	Version string
}

type Logger func(...interface{})
