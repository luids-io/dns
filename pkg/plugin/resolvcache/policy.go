// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package resolvcache

import (
	"errors"
	"fmt"
	"strings"

	"github.com/luids-io/api/event"
)

// RuleSet stores the policy rules
type RuleSet struct {
	MaxClientRequests Rule
	MaxNamesResolved  Rule
}

// Rule information
type Rule struct {
	Event EventInfo
	Log   bool
}

// EventInfo stores event information
type EventInfo struct {
	Raise bool
	Level event.Level
}

// ToRule returns rule information from string
func ToRule(s string) (Rule, error) {
	var rule Rule
	var err error
	//load fields an values to rule
	s = strings.ToLower(s)
	args := strings.Split(s, ",")
	for _, arg := range args {
		tmps := strings.SplitN(arg, "=", 2)
		field := strings.TrimSpace(tmps[0])
		value := ""
		if len(tmps) > 1 {
			value = strings.TrimSpace(tmps[1])
		}
		switch field {
		case "log":
			if value != "true" && value != "false" {
				return rule, fmt.Errorf("invalid log '%s'", value)
			}
			if value == "true" {
				rule.Log = true
			}
		case "event":
			rule.Event, err = ToEventInfo(value)
			if err != nil {
				return rule, err
			}
		}
	}
	return rule, nil
}

// ToEventInfo returns event information from string
func ToEventInfo(value string) (EventInfo, error) {
	var r EventInfo
	var err error
	switch strings.ToLower(value) {
	case "none":
		r.Raise = false
	case "info":
		r.Raise = true
		r.Level = event.Info
	case "low":
		r.Raise = true
		r.Level = event.Low
	case "medium":
		r.Raise = true
		r.Level = event.Medium
	case "high":
		r.Raise = true
		r.Level = event.High
	case "critical":
		r.Raise = true
		r.Level = event.Critical
	default:
		err = errors.New("invalid event format")
	}
	return r, err
}
