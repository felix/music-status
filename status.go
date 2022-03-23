package mstatus

import (
	"fmt"
	"time"
)

type Song struct {
	Title    string
	Artist   string
	Duration time.Duration
}

func (s Song) String() string {
	return fmt.Sprintf("%q by %s", s.Title, s.Artist)

}

type Status struct {
	Track Song
	Error error
}
type Logger func(...interface{})
