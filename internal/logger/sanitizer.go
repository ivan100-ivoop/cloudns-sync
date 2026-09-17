package logger

import "regexp"

var sensitive = regexp.MustCompile(`(?i)(auth[_-]?password|password|api[_-]?key|token|authorization|secret)(\s*[=:]\s*)([^\s,;]+)`)

func Sanitize(message string) string {
	return sensitive.ReplaceAllString(message, "$1=[REDACTED]")
}
