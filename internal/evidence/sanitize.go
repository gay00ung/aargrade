package evidence

import (
	"regexp"
	"sort"
	"strings"
)

var (
	sensitiveNamePattern  = regexp.MustCompile(`(?i)(?:password|passwd|passphrase|token|secret|credential|api[-_.]?key|private[-_.]?key)`)
	sensitiveValuePattern = regexp.MustCompile(`(?i)((?:password|passwd|passphrase|token|secret|credential|api[-_.]?key|private[-_.]?key)[^=\s]*=)[^\s]+`)
)

// Arguments returns a copy suitable for reports. It preserves ordinary Gradle
// arguments while masking common property and option names used for secrets.
func Arguments(arguments []string) []string {
	result := append([]string(nil), arguments...)
	maskNext := false
	for index, argument := range result {
		if maskNext {
			result[index] = "<redacted>"
			maskNext = false
			continue
		}
		key, _, hasValue := strings.Cut(argument, "=")
		name := strings.TrimLeft(key, "-")
		propertyStyle := !strings.HasPrefix(key, "--") && (strings.HasPrefix(key, "-P") || strings.HasPrefix(key, "-D"))
		if propertyStyle {
			name = key[2:]
		}
		if !sensitiveNamePattern.MatchString(name) {
			continue
		}
		if hasValue {
			result[index] = key + "=<redacted>"
		} else if !propertyStyle {
			maskNext = true
		}
	}
	return result
}

// Text masks common key=value secrets and replaces absolute evidence paths.
// Longer paths are replaced first so nested paths cannot be partially exposed.
func Text(value string, replacements map[string]string) string {
	value = sensitiveValuePattern.ReplaceAllString(value, `${1}<redacted>`)
	type replacement struct {
		from string
		to   string
	}
	items := make([]replacement, 0, len(replacements))
	for from, to := range replacements {
		if from != "" {
			items = append(items, replacement{from: from, to: to})
		}
	}
	sort.Slice(items, func(i, j int) bool { return len(items[i].from) > len(items[j].from) })
	for _, item := range items {
		value = strings.ReplaceAll(value, item.from, item.to)
	}
	return strings.TrimSpace(value)
}
