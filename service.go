package mstatus

type Handler interface {
	Handle(State, Status) error
}
type Source interface {
	Watch([]Handler) error
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
	return s.src.Watch(s.handlers)
}
