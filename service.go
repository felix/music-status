package mstatus

import (
	"fmt"
	"strings"
)

type Handler interface {
	Plugin
	Start(<-chan Status)
}
type Source interface {
	Plugin
	Watch() error
	Events() chan Status
	Stop() error
}

type Server struct {
	log      Logger
	src      Source
	handlers []Handler
}

func New(cfg *Config, opts ...Option) (*Server, error) {
	out := &Server{
		log: func(...interface{}) {},
	}

	for _, opt := range opts {
		if err := opt(out); err != nil {
			return nil, err
		}
	}

	sourceName := cfg.ReadString("global", "source")
	if sourceName == "" {
		return nil, fmt.Errorf("source not defined")
	}

	src, ok := getPlugin(sourceName).(Source)
	if src == nil || !ok {
		return nil, fmt.Errorf("source plugin not invalid")
	}
	out.log("loading source", src.Name())
	if err := src.Load(*cfg, out.log); err != nil {
		return nil, fmt.Errorf("failed to load source plugin")
	}
	out.src = src

	targetNames := strings.Split(cfg.ReadString("global", "targets"), ",")

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
			if err := h.Load(*cfg, out.log); err != nil {
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
	return s.src.Stop()
}
