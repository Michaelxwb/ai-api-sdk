package streaming

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ExtractJSONPath extracts the value at a JSON path from data.
// Path formats:
//   - "field" (top-level field)
//   - "field.subfield" (nested field)
//   - "field.0.subfield" (array index)
//   - "choices.0.delta.content" (combined path)
//   - "choices[0].delta.content" (bracket array index)
func ExtractJSONPath(data []byte, path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("json path is empty")
	}

	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return "", err
	}

	parts, err := splitJSONPath(path)
	if err != nil {
		return "", err
	}

	cur := root
	for _, part := range parts {
		switch v := cur.(type) {
		case map[string]any:
			next, ok := v[part]
			if !ok {
				return "", fmt.Errorf("json path not found: %s", path)
			}
			cur = next
		case []any:
			idx, err := strconv.Atoi(part)
			if err != nil {
				return "", fmt.Errorf("json path expects array index at %q", part)
			}
			if idx < 0 || idx >= len(v) {
				return "", fmt.Errorf("json path index out of range: %d", idx)
			}
			cur = v[idx]
		default:
			return "", fmt.Errorf("json path cannot traverse %T at %q", cur, part)
		}
	}

	return jsonValueToString(cur)
}

// MakeJSONPathExtractor builds a delta extractor based on a single JSON path.
func MakeJSONPathExtractor(deltaPath, donePath, doneValue string) DeltaExtractor {
	return MakeJSONPathMultiExtractor([]string{deltaPath}, donePath, doneValue)
}

// MakeJSONPathMultiExtractor builds a delta extractor based on multiple JSON paths.
func MakeJSONPathMultiExtractor(deltaPaths []string, donePath, doneValue string) DeltaExtractor {
	return func(event string, data []byte) (text string, done bool, err error) {
		if len(deltaPaths) > 0 {
			var builder strings.Builder
			for _, path := range deltaPaths {
				if path == "" {
					continue
				}
				val, err := ExtractJSONPath(data, path)
				if err != nil || val == "" {
					continue
				}
				builder.WriteString(val)
			}
			text = builder.String()
		}
		if donePath != "" {
			if val, err := ExtractJSONPath(data, donePath); err == nil {
				done = (val == doneValue)
			}
		}
		return text, done, nil
	}
}

func splitJSONPath(path string) ([]string, error) {
	parts := make([]string, 0, 8)
	var buf strings.Builder
	for i := 0; i < len(path); i++ {
		ch := path[i]
		switch ch {
		case '.':
			if buf.Len() == 0 {
				if len(parts) == 0 {
					return nil, fmt.Errorf("invalid json path: %q", path)
				}
				continue
			}
			parts = append(parts, buf.String())
			buf.Reset()
		case '[':
			if buf.Len() > 0 {
				parts = append(parts, buf.String())
				buf.Reset()
			}
			j := i + 1
			if j >= len(path) {
				return nil, fmt.Errorf("invalid json path: %q", path)
			}
			for ; j < len(path) && path[j] != ']'; j++ {
				buf.WriteByte(path[j])
			}
			if j >= len(path) || path[j] != ']' {
				return nil, fmt.Errorf("invalid json path: %q", path)
			}
			parts = append(parts, buf.String())
			buf.Reset()
			i = j
		case ']':
			return nil, fmt.Errorf("invalid json path: %q", path)
		default:
			buf.WriteByte(ch)
		}
	}
	if buf.Len() > 0 {
		parts = append(parts, buf.String())
	}
	if len(parts) == 0 {
		return nil, fmt.Errorf("invalid json path: %q", path)
	}
	return parts, nil
}

func jsonValueToString(value any) (string, error) {
	switch v := value.(type) {
	case nil:
		return "", nil
	case string:
		return v, nil
	case json.Number:
		return v.String(), nil
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	case bool:
		return strconv.FormatBool(v), nil
	case []byte:
		return string(v), nil
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
}
