package mstatus

import (
	"bytes"
	"encoding/csv"
	"encoding/gob"
	"fmt"
	"io"
	"strconv"
	"sync"
)

type Session struct {
	sync.Mutex
	data  map[string]string
	state map[string][]byte
	log   Logger
}

func readConfig(f io.Reader) (*Session, error) {
	r := csv.NewReader(f)
	r.Comma = '='
	r.Comment = '#'

	out := &Session{
		data:  make(map[string]string),
		state: make(map[string][]byte),
	}
	out.Lock()
	defer out.Unlock()

	data, err := r.ReadAll()
	if err != nil {
		return nil, err
	}

	for _, row := range data {
		out.data[row[0]] = row[1]
	}
	return out, nil
}

func (s *Session) ConfigString(scope, key string) string {
	s.Lock()
	defer s.Unlock()
	v, ok := s.data[scope+"."+key]
	if ok {
		return v
	}
	return ""
}

func (s *Session) ConfigInt(scope, key string) int {
	var out int
	if s := s.ConfigString(scope, key); s != "" {
		out, _ = strconv.Atoi(s)
	}
	return out
}

func (s *Session) WriteState(scope string, v any) error {
	s.Lock()
	defer s.Unlock()

	buf := new(bytes.Buffer)
	enc := gob.NewEncoder(buf)
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("failed to encode state for %q: %w", scope, err)
	}
	s.state[scope] = buf.Bytes()
	fmt.Printf("wrote state for %q\n", scope)
	return nil
}

func (s *Session) ReadState(scope string, v any) error {
	s.Lock()
	defer s.Unlock()

	b, ok := s.state[scope]
	if !ok {
		fmt.Println("no state", scope)
		return nil
	}
	dec := gob.NewDecoder(bytes.NewReader(b))
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("failed to decode state for %q: %w", scope, err)
	}
	return nil
}
