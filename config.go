package mstatus

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	data [][]string
}

func ReadConfig(p string) (*Config, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, fmt.Errorf("failed to read config %q: %s\n", p, err)
	}
	r := csv.NewReader(f)
	r.Comma = '='
	r.Comment = '#'

	out := new(Config)

	out.data, err = r.ReadAll()
	return out, err
}

func (cfg *Config) ReadString(scope, key string) string {
	for _, row := range cfg.data {
		parts := strings.SplitN(row[0], ".", 2)
		if strings.EqualFold(parts[0], scope) && strings.EqualFold(parts[1], key) {
			return row[1]
		}
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
