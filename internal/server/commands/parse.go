// parse.go implements pure client command string-parsing helpers.
package commands

import (
	"strconv"
	"strings"
)

// ClientStringCommandVerb returns the lowercased first whitespace-delimited
// token of a client command string, or "" if empty.
func ClientStringCommandVerb(cmd string) string {
	fields := strings.Fields(strings.TrimSpace(cmd))
	if len(fields) == 0 {
		return ""
	}
	return strings.ToLower(fields[0])
}

// ClientStringCommandArgs returns the portion of the command string after the
// verb, trimmed, or "" if there is no verb.
func ClientStringCommandArgs(cmd string) string {
	trimmed := strings.TrimSpace(cmd)
	verb := ClientStringCommandVerb(trimmed)
	if verb == "" {
		return ""
	}
	return strings.TrimSpace(trimmed[len(verb):])
}

// ParseClientNameCommand extracts the player name from a "name" command,
// unquoting if needed and truncating to 15 characters.
func ParseClientNameCommand(cmd string) string {
	value := ClientStringCommandArgs(cmd)
	if value == "" {
		return ""
	}
	if unquoted, err := strconv.Unquote(value); err == nil {
		value = unquoted
	}
	if len(value) > 15 {
		value = value[:15]
	}
	return value
}

// ParseClientColorCommand extracts the top/bottom color pair from a "color"
// command as top*16+bottom.
func ParseClientColorCommand(cmd string) int {
	args := strings.Fields(ClientStringCommandArgs(cmd))
	if len(args) == 0 {
		return 0
	}
	top, _ := strconv.Atoi(args[0])
	if len(args) == 1 {
		return top
	}
	bottom, _ := strconv.Atoi(args[1])
	top &= 15
	bottom &= 15
	return top*16 + bottom
}