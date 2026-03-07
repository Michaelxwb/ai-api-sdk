package generic

import (
	"encoding/json"
	"fmt"
	"strings"
)

// renderTemplate renders a body template by replacing placeholders.
// Supported placeholders:
//   - {{input}}:        user input (JSON-escaped)
//   - {{session_id}}:   remote session ID; empty → field removed
//   - $$$NAME$$$:       chain field value from chainValues; empty → field removed
func renderTemplate(tmpl map[string]any, input string, sessionID string, chainValues map[string]string) (string, error) {
	data, err := json.Marshal(tmpl)
	if err != nil {
		return "", fmt.Errorf("generic: failed to marshal template: %w", err)
	}

	tplStr := string(data)

	// {{input}}: JSON-safe string (content without surrounding quotes)
	if strings.Contains(tplStr, "{{input}}") {
		escaped, err := jsonEscapeString(input)
		if err != nil {
			return "", fmt.Errorf("generic: failed to escape input: %w", err)
		}
		tplStr = strings.ReplaceAll(tplStr, "{{input}}", escaped)
	}

	// {{session_id}}: inject or remove field
	if strings.Contains(tplStr, "{{session_id}}") {
		if sessionID != "" {
			escaped, err := jsonEscapeString(sessionID)
			if err != nil {
				return "", fmt.Errorf("generic: failed to escape session_id: %w", err)
			}
			tplStr = strings.ReplaceAll(tplStr, "{{session_id}}", escaped)
		} else {
			// Remove the field containing {{session_id}} placeholder
			tplStr = removeSessionIDField(tplStr)
		}
	}

	// $$$NAME$$$: chain field placeholders — inject or remove field
	for placeholder, value := range chainValues {
		if !strings.Contains(tplStr, placeholder) {
			continue
		}
		if value != "" {
			escaped, err := jsonEscapeString(value)
			if err != nil {
				return "", fmt.Errorf("generic: failed to escape chain value for %s: %w", placeholder, err)
			}
			tplStr = strings.ReplaceAll(tplStr, placeholder, escaped)
		} else {
			tplStr = removeFieldWithValue(tplStr, placeholder)
		}
	}
	// Also remove any remaining $$$NAME$$$ placeholders that had no value provided
	tplStr = removeRemainingChainPlaceholders(tplStr)

	return tplStr, nil
}

// jsonEscapeString returns the JSON-escaped interior of a string (without surrounding quotes).
func jsonEscapeString(s string) (string, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	// Remove surrounding quotes
	inner := string(b[1 : len(b)-1])
	return inner, nil
}

// removeSessionIDField removes JSON fields that contain the {{session_id}} placeholder.
func removeSessionIDField(jsonStr string) string {
	return removeFieldWithValue(jsonStr, "{{session_id}}")
}

// removeFieldWithValue removes the JSON field whose string value equals placeholder.
func removeFieldWithValue(jsonStr string, placeholder string) string {
	var m map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &m); err != nil {
		// Fallback: simple string replacement
		return strings.ReplaceAll(jsonStr, placeholder, "")
	}
	removePlaceholderValue(m, placeholder)
	result, err := json.Marshal(m)
	if err != nil {
		return jsonStr
	}
	return string(result)
}

func removePlaceholderValue(m map[string]any, placeholder string) {
	for k, v := range m {
		switch val := v.(type) {
		case string:
			if val == placeholder {
				delete(m, k)
			}
		case map[string]any:
			removePlaceholderValue(val, placeholder)
		case []any:
			for _, item := range val {
				if subMap, ok := item.(map[string]any); ok {
					removePlaceholderValue(subMap, placeholder)
				}
			}
		}
	}
}

// removeRemainingChainPlaceholders removes any $$$NAME$$$ fields that were not substituted.
func removeRemainingChainPlaceholders(jsonStr string) string {
	matches := chainPlaceholderRe.FindAllString(jsonStr, -1)
	if len(matches) == 0 {
		return jsonStr
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &m); err != nil {
		return jsonStr
	}
	for _, ph := range matches {
		removePlaceholderValue(m, ph)
	}
	result, err := json.Marshal(m)
	if err != nil {
		return jsonStr
	}
	return string(result)
}
