package output

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"regexp"

	"github.com/fatih/color"
)

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

var (
	jsonMode    bool
	compactMode bool
	limitN      int
)

func SetJSON(v bool) {
	jsonMode = v
	if v {
		color.NoColor = true // never emit ANSI in JSON mode
	}
}
func SetCompact(v bool) { compactMode = v }
func SetLimit(v int)    { limitN = v }
func IsJSON() bool      { return jsonMode }
func Limit() int        { return limitN }

var (
	Red    = color.New(color.FgRed).SprintFunc()
	Green  = color.New(color.FgGreen).SprintFunc()
	Yellow = color.New(color.FgYellow).SprintFunc()
	Cyan   = color.New(color.FgCyan).SprintFunc()
	Bold   = color.New(color.Bold).SprintFunc()
	Dim    = color.New(color.Faint).SprintFunc()
)

// Print outputs data as JSON or formatted text.
// compactFn reduces data to essential fields (used with --compact).
func Print(data any, formatted string, compactFn func(any) any) {
	out := data
	if compactMode && compactFn != nil {
		out = applyCompact(out, compactFn)
	}
	if limitN > 0 {
		out = applyLimit(out, limitN)
	}
	if jsonMode {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(out)
	} else {
		fmt.Println(formatted)
	}
}

// Fail prints an error and exits 1.
func Fail(err error) {
	if jsonMode {
		_ = json.NewEncoder(os.Stderr).Encode(map[string]string{"error": err.Error()})
	} else {
		fmt.Fprintln(os.Stderr, Red("Error: ")+err.Error())
	}
	os.Exit(1)
}

func applyCompact(data any, fn func(any) any) any {
	v := reflect.ValueOf(data)
	if v.Kind() == reflect.Slice {
		result := make([]any, v.Len())
		for i := range v.Len() {
			result[i] = fn(v.Index(i).Interface())
		}
		return result
	}
	return fn(data)
}

// LimitSlice truncates a slice to at most n items (0 = no limit).
// Use this in commands to honour --limit on the inner items slice.
func LimitSlice[T any](s []T) []T {
	if limitN > 0 && len(s) > limitN {
		return s[:limitN]
	}
	return s
}

// kept for backward compat on raw slices passed directly to Print
func applyLimit(data any, n int) any {
	v := reflect.ValueOf(data)
	if v.Kind() == reflect.Slice && v.Len() > n {
		return v.Slice(0, n).Interface()
	}
	return data
}

// StatusColor returns colored status string for terminal.
func StatusColor(status string) string {
	switch status {
	case "PASSING", "PASSED", "ACTIVE", "COMPLIANT", "READY":
		return Green(status)
	case "FAILED", "NOT_READY", "NO_OWNER", "NEEDS_ATTENTION":
		return Red(status)
	case "NEEDS_EVIDENCE", "NOT_TESTED", "WARNING":
		return Yellow(status)
	case "ARCHIVED":
		return Dim(status)
	default:
		return status
	}
}

// Col formats a string in a fixed-width column, accounting for ANSI escape codes.
func Col(s string, width int) string {
	visible := len(ansiRe.ReplaceAllString(s, ""))
	if visible >= width {
		return s + " "
	}
	return s + fmt.Sprintf("%*s", width-visible, "")
}
