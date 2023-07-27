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

type Track struct {
	ID          string
	Title       string
	Artist      string
	Album       string
	Duration    time.Duration
	Elapsed     time.Duration
	MbArtistID  string
	MbTrackID   string // recording ID
	MbReleaseID string // album ID
}

func (s Track) String() string {
	return fmt.Sprintf("%q by %s", s.Title, s.Artist)

}

type Status struct {
	State  State
	Player Player
	Track  *Track
	Error  error
}

type Player struct {
	Name    string
	Version string
}

type Logger func(...any)

func prefixedLogger(prefix string, l Logger) Logger {
	if l == nil {
		return l
	}
	return func(v ...any) {
		l(append([]any{prefix}, v...)...)
	}
}
