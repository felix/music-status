package mstatus

type Handler interface {
	Start(<-chan Status)
}
type Source interface {
	Watch() error
	Events() chan Status
	Stop() error
}

type Server struct {
	log      Logger
	src      Source
	handlers []Handler
}

func New(src Source, opts ...Option) (*Server, error) {
	out := &Server{
		src: src,
		log: func(...interface{}) {},
	}
	for _, opt := range opts {
		if err := opt(out); err != nil {
			return nil, err
		}
	}
	return out, nil
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
