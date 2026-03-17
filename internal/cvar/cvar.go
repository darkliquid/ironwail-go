package cvar

import (
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"sync"
)

type CVarFlags int

const (
	FlagNone    CVarFlags = 0
	FlagArchive CVarFlags = 1 << iota
	FlagNotify
	FlagServerInfo
	FlagUserInfo
	FlagNoSet
	FlagLatched
	FlagROM
)

type CVar struct {
	Name         string
	String       string
	Float        float64
	Int          int
	Flags        CVarFlags
	DefaultValue string
	Description  string
	Callback     func(cv *CVar)
	modified     bool
}

// Bool returns the cvar value as a boolean.
// Returns true if Int != 0.
func (cv *CVar) Bool() bool {
	return cv.Int != 0
}

// Float32 returns the cvar value as a float32.
func (cv *CVar) Float32() float32 {
	return float32(cv.Float)
}

type CVarSystem struct {
	mu   sync.RWMutex
	vars map[string]*CVar
}

var globalCVar = NewCVarSystem()

func NewCVarSystem() *CVarSystem {
	return &CVarSystem{
		vars: make(map[string]*CVar),
	}
}

func (c *CVarSystem) Get(name string) *CVar {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.vars[strings.ToLower(name)]
}

func (c *CVarSystem) Register(name, defaultValue string, flags CVarFlags, desc string) *CVar {
	c.mu.Lock()
	defer c.mu.Unlock()

	name = strings.ToLower(name)
	if cv, exists := c.vars[name]; exists {
		return cv
	}

	cv := &CVar{
		Name:         name,
		String:       defaultValue,
		DefaultValue: defaultValue,
		Flags:        flags,
		Description:  desc,
		modified:     false,
	}
	c.parseValue(cv, defaultValue)
	c.vars[name] = cv
	return cv
}

func (c *CVarSystem) Set(name, value string) {
	name = strings.ToLower(name)
	c.mu.Lock()
	cv, exists := c.vars[name]
	if !exists {
		cv = &CVar{
			Name:   name,
			String: value,
			Flags:  FlagNone,
		}
		c.parseValue(cv, value)
		c.vars[name] = cv
		c.mu.Unlock()
		return
	}

	if cv.Flags&FlagNoSet != 0 {
		c.mu.Unlock()
		return
	}

	if cv.Flags&FlagROM != 0 {
		c.mu.Unlock()
		slog.Info("cvar is read-only", "name", name)
		return
	}

	if cv.Flags&FlagLatched != 0 {
		cv.String = value
		c.parseValue(cv, value)
		cv.modified = true
		c.mu.Unlock()
		return
	}

	cv.String = value
	c.parseValue(cv, value)
	cv.modified = true
	callback := cv.Callback
	c.mu.Unlock()

	if callback != nil {
		callback(cv)
	}
}

func (c *CVarSystem) SetFloat(name string, value float64) {
	c.Set(name, strconv.FormatFloat(value, 'f', -1, 64))
}

func (c *CVarSystem) SetInt(name string, value int) {
	c.Set(name, strconv.Itoa(value))
}

func (c *CVarSystem) SetBool(name string, value bool) {
	if value {
		c.Set(name, "1")
	} else {
		c.Set(name, "0")
	}
}

func (c *CVarSystem) parseValue(cv *CVar, value string) {
	cv.String = value

	if f, err := strconv.ParseFloat(value, 64); err == nil {
		cv.Float = f
		cv.Int = int(f)
	} else {
		cv.Float = 0
		cv.Int = 0
	}
}

func (c *CVarSystem) FloatValue(name string) float64 {
	if cv := c.Get(name); cv != nil {
		return cv.Float
	}
	return 0
}

func (c *CVarSystem) IntValue(name string) int {
	if cv := c.Get(name); cv != nil {
		return cv.Int
	}
	return 0
}

func (c *CVarSystem) BoolValue(name string) bool {
	return c.IntValue(name) != 0
}

func (c *CVarSystem) StringValue(name string) string {
	if cv := c.Get(name); cv != nil {
		return cv.String
	}
	return ""
}

func (c *CVarSystem) All() []*CVar {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]*CVar, 0, len(c.vars))
	for _, cv := range c.vars {
		result = append(result, cv)
	}
	return result
}

func (c *CVarSystem) ArchiveVars() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []string
	for _, cv := range c.vars {
		if cv.Flags&FlagArchive != 0 {
			result = append(result, cv.Name+" \""+cv.String+"\"")
		}
	}
	slices.Sort(result)
	return result
}

func (c *CVarSystem) Complete(partial string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	partial = strings.ToLower(partial)
	var matches []string
	for name := range c.vars {
		if strings.HasPrefix(name, partial) {
			matches = append(matches, name)
		}
	}
	return matches
}

func Get(name string) *CVar {
	return globalCVar.Get(name)
}

func Register(name, defaultValue string, flags CVarFlags, desc string) *CVar {
	return globalCVar.Register(name, defaultValue, flags, desc)
}

func Set(name, value string) {
	globalCVar.Set(name, value)
}

func SetFloat(name string, value float64) {
	globalCVar.SetFloat(name, value)
}

func SetInt(name string, value int) {
	globalCVar.SetInt(name, value)
}

func SetBool(name string, value bool) {
	globalCVar.SetBool(name, value)
}

func FloatValue(name string) float64 {
	return globalCVar.FloatValue(name)
}

func IntValue(name string) int {
	return globalCVar.IntValue(name)
}

func BoolValue(name string) bool {
	return globalCVar.BoolValue(name)
}

func StringValue(name string) string {
	return globalCVar.StringValue(name)
}

func All() []*CVar {
	return globalCVar.All()
}

func ArchiveVars() []string {
	return globalCVar.ArchiveVars()
}

func Complete(partial string) []string {
	return globalCVar.Complete(partial)
}
