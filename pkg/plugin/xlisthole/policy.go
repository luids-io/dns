// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package xlisthole

import (
	"errors"
	"fmt"
	"strings"

	"github.com/luids-io/api/event"
	"github.com/luids-io/core/reason"
)

// RuleSet stores the policy items.
type RuleSet struct {
	Domain  Rules
	IP      Rules
	CNAME   Rules
	OnError ActionInfo
}

// Rules struct groups rules for each item in policy.
type Rules struct {
	Listed   Rule
	Unlisted Rule
	Merge    bool
}

// Rule stores rule information.
type Rule struct {
	Action ActionInfo
	Event  EventInfo
	Log    bool
}

// ActionInfo stores dns action information.
type ActionInfo struct {
	Type ActionType
	Data string
}

// ActionType defines actions for the rules.
type ActionType int

//ActionType values.
const (
	SendNXDomain ActionType = iota
	SendFixedIP4
	SendRefused
	ReturnValue
	CheckIP
	CheckCNAME
	CheckAll
)

func (a ActionType) String() string {
	switch a {
	case SendNXDomain:
		return "nxdomain"
	case SendFixedIP4:
		return "ip"
	case SendRefused:
		return "refused"
	case ReturnValue:
		return "return"
	case CheckIP:
		return "checkip"
	case CheckCNAME:
		return "checkcname"
	case CheckAll:
		return "check"
	}
	return "unknown"
}

// EventInfo stores event information in rules.
type EventInfo struct {
	Raise bool
	Level event.Level
}

// Validate validates ruleset.
func (r RuleSet) Validate() error {
	err := r.Domain.Listed.Validate()
	if err != nil {
		return fmt.Errorf("invalid listed-domain: %v", err)
	}
	err = r.Domain.Unlisted.Validate()
	if err != nil {
		return fmt.Errorf("invalid unlisted-domain: %v", err)
	}
	err = r.IP.Listed.Validate()
	if err != nil {
		return fmt.Errorf("invalid listed-ip: %v", err)
	}
	if r.IP.Listed.Action.Type > ReturnValue {
		return fmt.Errorf("invalid listed-ip: %v", r.IP.Listed.Action.Type)
	}
	err = r.IP.Unlisted.Validate()
	if err != nil {
		return fmt.Errorf("invalid unlisted-ip: %v", err)
	}
	if r.IP.Unlisted.Action.Type > ReturnValue {
		return fmt.Errorf("invalid unlisted-ip: %v", r.IP.Unlisted.Action.Type)
	}
	err = r.CNAME.Listed.Validate()
	if err != nil {
		return fmt.Errorf("invalid listed-cname: %v", err)
	}
	if r.CNAME.Listed.Action.Type > ReturnValue {
		return fmt.Errorf("invalid listed-cname: %v", r.CNAME.Listed.Action.Type)
	}
	err = r.CNAME.Unlisted.Validate()
	if err != nil {
		return fmt.Errorf("invalid unlisted-cname: %v", err)
	}
	if r.CNAME.Unlisted.Action.Type > ReturnValue {
		return fmt.Errorf("invalid unlisted-cname: %v", r.CNAME.Unlisted.Action.Type)
	}
	if r.OnError.Type > ReturnValue {
		return fmt.Errorf("invalid on-error: %v", r.OnError.Type)
	}
	return nil
}

// Validate validates a rule.
func (r Rule) Validate() error {
	if r.Action.Type == SendFixedIP4 {
		if !isIPv4(r.Action.Data) {
			return errors.New("invalid ip in action")
		}
	}
	return nil
}

// applyPolicy applies policy values to the rule
func (r *Rule) applyPolicy(p reason.Policy) error {
	avalue, ok := p.Get("dns")
	if ok {
		action, err := ToActionInfo(avalue)
		if err != nil {
			return err
		}
		r.Action = action
	}
	logvalue, ok := p.Get("log")
	if ok {
		switch strings.ToLower(logvalue) {
		case "true":
			r.Log = true
		case "false":
			r.Log = false
		default:
			return fmt.Errorf("invalid log '%s'", logvalue)
		}
	}
	eventvalue, ok := p.Get("event")
	if ok {
		event, err := ToEventInfo(eventvalue)
		if err != nil {
			return err
		}
		r.Event = event
	}
	return nil
}

// Merge extracted reason.
func (r *Rule) Merge(s string) error {
	p, _, err := reason.ExtractPolicy(s)
	if err != nil {
		return fmt.Errorf("processing reason '%s': %v", s, err)
	}
	err = r.applyPolicy(p)
	if err != nil {
		return fmt.Errorf("applying policy '%s': %v", s, err)
	}
	return nil
}

// ToRule returns rule information from string.
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
		case "dns":
			rule.Action, err = ToActionInfo(value)
			if err != nil {
				return rule, err
			}
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

// ToActionInfo returns an action from string.
func ToActionInfo(s string) (ActionInfo, error) {
	var r ActionInfo
	switch strings.ToLower(s) {
	case "nxdomain":
		r.Type = SendNXDomain
		return r, nil
	case "refused":
		r.Type = SendRefused
		return r, nil
	case "return":
		r.Type = ReturnValue
		return r, nil
	case "checkip":
		r.Type = CheckIP
		return r, nil
	case "checkcname":
		r.Type = CheckCNAME
		return r, nil
	case "check":
		r.Type = CheckAll
		return r, nil
	}
	if strings.HasPrefix(strings.ToLower(s), "ip:") {
		sip := s[3:]
		if !isIPv4(sip) {
			return r, fmt.Errorf("invalid ip '%s'", sip)
		}
		r.Type = SendFixedIP4
		r.Data = sip
		return r, nil
	}
	return r, errors.New("invalid action")
}

// ToEventInfo returns event information from string.
func ToEventInfo(s string) (EventInfo, error) {
	var r EventInfo
	var err error
	switch strings.ToLower(s) {
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
