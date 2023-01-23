package mstatus

import (
	"strings"
	"sync"
)

var (
	pluginsMu sync.RWMutex
	plugins   = []Plugin{}
)

type Plugin interface {
	Name() string
	Load(Config, Logger) error
	//Run() error
}

func Register(p Plugin) {
	if p == nil {
		panic("nil plugin")
	}
	pluginsMu.Lock()
	defer pluginsMu.Unlock()
	plugins = append(plugins, p)
}

func listPlugins() []string {
	pluginsMu.Lock()
	defer pluginsMu.Unlock()
	var out []string
	for _, p := range plugins {
		out = append(out, p.Name())
	}
	return out
}

func getPlugin(name string) Plugin {
	pluginsMu.Lock()
	defer pluginsMu.Unlock()
	for _, p := range plugins {
		if strings.EqualFold(p.Name(), name) {
			return p
		}
	}
	return nil
}
