package config

import (
	"fmt"
	"strings"
)

type OpenWith struct {
	Name string `toml:"name"`
	Cmd  string `toml:"cmd"`
}

func (c *Config) ValidOpenWith() ([]OpenWith, []string) {
	var valid []OpenWith
	var problems []string

	for i, entry := range c.OpenWith {
		name := strings.TrimSpace(entry.Name)
		if name == "" || strings.TrimSpace(entry.Cmd) == "" {
			problems = append(problems, fmt.Sprintf(
				"OpenWith entry %d needs a name and a cmd and was left out", i+1))
			continue
		}
		valid = append(valid, OpenWith{Name: name, Cmd: entry.Cmd})
	}

	return valid, problems
}
