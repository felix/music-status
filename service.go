package mstatus

import (
	"encoding/gob"
	"fmt"
	"log"
	"os"
	"path"
	"strings"
	"sync/atomic"
)

type Handler interface {
	Plugin
	Start(<-chan Status)
}
type Source interface {
	Plugin
	Watch() error
	Events() chan Status
}

type Server struct {
	log           Logger
	src           Source
	handlers      []Handler
	stateFilePath string
	stopping      atomic.Bool
	sess          *Session
}

func New(opts ...Option) (*Server, error) {
	cfgPath, err := os.UserConfigDir()
	if err != nil {
		log.Fatal(err)
	}
	cfgPath = path.Join(cfgPath, "music-status")
	if err := os.MkdirAll(cfgPath, 0775); err != nil {
		log.Fatal(err)
	}
	cfgFile, err := os.Open(path.Join(cfgPath, "config"))
	if err != nil {
		log.Fatal(err)
	}
	defer cfgFile.Close()

	sess, err := readConfig(cfgFile)
	if err != nil {
		return nil, err
	}

	stateFilePath := path.Join(cfgPath, "state")
	stateFile, err := os.Open(stateFilePath)
	if err == nil {
		defer stateFile.Close()
		dec := gob.NewDecoder(stateFile)
		if err := dec.Decode(&sess.state); err != nil {
			log.Printf("failed to decode state file: %s", err)
		}
	}

	out := &Server{
		log:           func(...any) {},
		sess:          sess,
		stateFilePath: stateFilePath,
	}

	for _, opt := range opts {
		if err := opt(out); err != nil {
			return nil, err
		}
	}

	sourceName := sess.ConfigString("global", "source")
	if sourceName == "" {
		return nil, fmt.Errorf("source not defined")
	}

	src, ok := getPlugin(sourceName).(Source)
	if src == nil || !ok {
		return nil, fmt.Errorf("source plugin invalid")
	}
	out.log("loading source", src.Name())
	if err := src.Load(sess, out.log); err != nil {
		return nil, fmt.Errorf("failed to load source plugin")
	}
	out.src = src

	targetNames := strings.Split(sess.ConfigString("global", "targets"), ",")

	for _, n := range listPlugins() {
		if strings.EqualFold(n, sourceName) {
			continue
		}
		if len(targetNames) == 0 || contains(n, targetNames) {
			tgt := getPlugin(n).(Handler)
			if tgt == nil {
				return nil, fmt.Errorf("target %q invalid", n)
			}
			h, ok := tgt.(Handler)
			if !ok {
				return nil, fmt.Errorf("target %q invalid", n)
			}
			if err := h.Load(out.sess, prefixedLogger(h.Name(), out.log)); err != nil {
				return nil, fmt.Errorf("failed to load target plugin %q", n)
			}
			out.handlers = append(out.handlers, tgt)
		}
	}

	return out, nil
}

func contains(needle string, haystack []string) bool {
	for _, s := range haystack {
		if strings.EqualFold(s, needle) {
			return true
		}
	}
	return false
}

type Option option

type option func(*Server) error

func WithHandler(h Handler) Option {
	return func(s *Server) error {
		s.handlers = append(s.handlers, h)
		return nil
	}
}

func WithLogger(l Logger) Option {
	return func(s *Server) error {
		s.log = l
		return nil
	}
}

func (s *Server) Start() error {
	var pub []chan Status
	for _, h := range s.handlers {
		ch := make(chan Status)
		pub = append(pub, ch)
		go h.Start(ch)
	}

	go func() {
		var lastState State
		for event := range s.src.Events() {
			if event.State != lastState {
				s.log("server event:", event.State)
				lastState = event.State
			}
			for _, ch := range pub {
				ch <- event
			}
		}
	}()

	// Blocks
	return s.src.Watch()
}

func (s *Server) Stop() error {
	if s.stopping.Load() {
		return nil
	}
	s.stopping.Store(true)
	s.log("service stopping")
	for _, h := range s.handlers {
		s.log("stopping plugin", h.Name())
		if err := h.Stop(); err != nil {
			s.log("failed to stop plugin", h.Name(), err)
		}
	}
	if err := s.src.Stop(); err != nil {
		s.log("failed to stop source", s.src.Name(), err)
	}

	// Write out state file
	s.log("writing state file")
	stateFile, err := os.Create(s.stateFilePath)
	if err != nil {
		s.log("failed to open state file", err)
	}
	defer stateFile.Close()

	s.sess.Lock()
	defer s.sess.Unlock()
	enc := gob.NewEncoder(stateFile)
	if err := enc.Encode(s.sess.state); err != nil {
		s.log("failed to encode state file", err)
	}
	return nil
}
