package mstatus

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	data map[string]string
}

func ReadConfig(p string) (*Config, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, fmt.Errorf("failed to read config %q: %s\n", p, err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '='
	r.Comment = '#'

	out := new(Config)

	data, err := r.ReadAll()
	if err != nil {
		return nil, err
	}

	out.data = make(map[string]string)

	for _, row := range data {
		out.data[row[0]] = row[1]
	}
	return out, nil
}

func (cfg *Config) ReadString(scope, key string) string {
	v, ok := cfg.data[scope+"."+key]
	if ok {
		return v
	}
	return ""
}

func (cfg *Config) ReadInt(scope, key string) int {
	var out int
	if s := cfg.ReadString(scope, key); s != "" {
		out, _ = strconv.Atoi(s)
	}
	return out
}
