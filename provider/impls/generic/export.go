package generic

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// ExportToHTTPSpec converts an InferredIntegration (from multi-round inference)
// back to a RawHTTPSpec suitable for persisting or passing to Quick API.
//
// Parameters:
//   - inferred: the result of InferIntegrationByMultiRound or similar
//   - rawSpec: the original RawHTTPMultiRoundSpec used for inference
//     (needed to extract the first round's raw request headers and response text)
func ExportToHTTPSpec(inferred *InferredIntegration, rawSpec RawHTTPMultiRoundSpec) (*RawHTTPSpec, error) {
	if inferred == nil || inferred.Profile == nil {
		return nil, fmt.Errorf("generic: inferred integration is nil, cannot export")
	}
	if len(rawSpec.Rounds) == 0 {
		return nil, fmt.Errorf("generic: rounds is empty, cannot export")
	}

	method, headers, err := exportParseRequestMeta(rawSpec.Rounds[0].Request)
	if err != nil {
		return nil, fmt.Errorf("generic: %w", err)
	}

	requestPath := strings.TrimSpace(inferred.Profile.Request.Path)
	if requestPath == "" {
		requestPath = exportResolvePathFromURL(rawSpec.BaseURL)
	}
	if requestPath == "" {
		requestPath = "/"
	}

	chainFields := append([]ChainField(nil), inferred.Profile.Response.Stream.ChainFields...)
	bodyTemplate, chainFields := exportNormalizeBodyTemplate(inferred.Profile.Request.BodyTemplate, rawSpec.Rounds, chainFields)

	bodyText, err := exportRenderBody(bodyTemplate)
	if err != nil {
		return nil, fmt.Errorf("generic: %w", err)
	}

	requestText := exportBuildRequestText(method, requestPath, headers, bodyText)
	mode := exportInferConversationMode(strings.TrimSpace(rawSpec.Model), inferred)

	spec := &RawHTTPSpec{
		Model:       mode,
		BaseURL:     exportResolveBaseURL(rawSpec.BaseURL, inferred.BaseURL, requestPath),
		Request:     requestText,
		Response:    rawSpec.Rounds[0].Response,
		ChainFields: chainFields,
	}
	return spec, nil
}

// --- unexported helpers ---

type exportHeader struct {
	Key   string
	Value string
}

func exportParseRequestMeta(rawRequest string) (string, []exportHeader, error) {
	text := strings.ReplaceAll(rawRequest, "\r\n", "\n")
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		return "", nil, fmt.Errorf("first round request is empty")
	}

	first := ""
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		first = strings.TrimSpace(line)
		break
	}
	if first == "" {
		return "", nil, fmt.Errorf("first round request missing request line")
	}

	parts := strings.Fields(first)
	if len(parts) == 0 {
		return "", nil, fmt.Errorf("first round request line is invalid")
	}
	method := strings.ToUpper(strings.TrimSpace(parts[0]))
	if method == "" {
		method = "POST"
	}

	var headers []exportHeader
	seen := map[string]struct{}{}
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "" {
			break
		}
		idx := strings.Index(line, ":")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		if key == "" {
			continue
		}
		if strings.EqualFold(key, "Content-Length") {
			continue
		}
		value := strings.TrimSpace(line[idx+1:])
		lower := strings.ToLower(key)
		if _, ok := seen[lower]; ok {
			continue
		}
		seen[lower] = struct{}{}
		headers = append(headers, exportHeader{Key: key, Value: value})
	}

	sort.SliceStable(headers, func(i, j int) bool {
		return strings.ToLower(headers[i].Key) < strings.ToLower(headers[j].Key)
	})
	return method, headers, nil
}

func exportNormalizeBodyTemplate(bodyTemplate map[string]any, rounds []RawHTTPRound, chainFields []ChainField) (map[string]any, []ChainField) {
	body := exportCloneMap(bodyTemplate)
	if body == nil {
		body = map[string]any{}
	}

	reqChainIdx := -1
	for i, cf := range chainFields {
		if strings.EqualFold(strings.TrimSpace(cf.ResponsePath), "req_id") && strings.TrimSpace(cf.Placeholder) != "" {
			reqChainIdx = i
			break
		}
	}

	if reqChainIdx >= 0 {
		body["parent_req_id"] = chainFields[reqChainIdx].Placeholder
	} else if ph, ok := body["parent_req_id"].(string); ok && strings.TrimSpace(ph) != "" {
		chainFields = append(chainFields, ChainField{
			Placeholder:  ph,
			ResponsePath: "req_id",
		})
	}

	if exportHasReqIDInAnyRound(rounds) {
		body["req_id"] = "{{uuid}}"
	}

	return body, chainFields
}

func exportHasReqIDInAnyRound(rounds []RawHTTPRound) bool {
	for _, round := range rounds {
		if exportHasTopLevelJSONField(round.Request, "req_id") {
			return true
		}
	}
	return false
}

func exportHasTopLevelJSONField(rawRequest, field string) bool {
	body, err := exportExtractRequestBody(rawRequest)
	if err != nil || body == "" {
		return false
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		return false
	}
	_, ok := parsed[field]
	return ok
}

func exportExtractRequestBody(rawRequest string) (string, error) {
	text := strings.ReplaceAll(rawRequest, "\r\n", "\n")
	parts := strings.SplitN(text, "\n\n", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("raw request missing body section")
	}
	return strings.TrimSpace(parts[1]), nil
}

func exportRenderBody(bodyTemplate map[string]any) (string, error) {
	rawBody := exportNormalizeIndexedObjectToArray(exportToRawPlaceholders(bodyTemplate))
	data, err := json.MarshalIndent(rawBody, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal export body failed: %w", err)
	}
	return string(data), nil
}

func exportToRawPlaceholders(v any) any {
	switch val := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(val))
		for k, item := range val {
			out[k] = exportToRawPlaceholders(item)
		}
		return out
	case []any:
		out := make([]any, len(val))
		for i, item := range val {
			out[i] = exportToRawPlaceholders(item)
		}
		return out
	case string:
		s := strings.ReplaceAll(val, "{{input}}", "$$$")
		s = strings.ReplaceAll(s, "{{session_id}}", "$$$SESSION_ID$$$")
		return s
	default:
		return v
	}
}

func exportNormalizeIndexedObjectToArray(v any) any {
	switch val := v.(type) {
	case map[string]any:
		normalized := make(map[string]any, len(val))
		for k, item := range val {
			normalized[k] = exportNormalizeIndexedObjectToArray(item)
		}
		if arr, ok := exportMapWithContiguousIndexToArray(normalized); ok {
			return arr
		}
		return normalized
	case []any:
		out := make([]any, len(val))
		for i, item := range val {
			out[i] = exportNormalizeIndexedObjectToArray(item)
		}
		return out
	default:
		return v
	}
}

func exportMapWithContiguousIndexToArray(m map[string]any) ([]any, bool) {
	if len(m) == 0 {
		return nil, false
	}

	indexed := make([]any, len(m))
	for key, value := range m {
		idx, err := strconv.Atoi(key)
		if err != nil || idx < 0 || idx >= len(m) {
			return nil, false
		}
		indexed[idx] = value
	}
	for i := range indexed {
		if indexed[i] == nil {
			return nil, false
		}
	}
	return indexed, true
}

func exportBuildRequestText(method, path string, headers []exportHeader, body string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s HTTP/1.1\n", method, path)
	for _, h := range headers {
		fmt.Fprintf(&b, "%s: %s\n", h.Key, h.Value)
	}
	b.WriteString("\n")
	b.WriteString(body)
	return b.String()
}

func exportResolveBaseURL(rawBaseURL, inferredBaseURL, requestPath string) string {
	if strings.TrimSpace(rawBaseURL) != "" {
		return strings.TrimSpace(rawBaseURL)
	}
	base := strings.TrimSpace(inferredBaseURL)
	path := strings.TrimSpace(requestPath)
	if base == "" {
		return path
	}
	if path == "" {
		return base
	}
	if strings.HasPrefix(path, "/") || strings.HasPrefix(path, "?") {
		return strings.TrimRight(base, "/") + path
	}
	return strings.TrimRight(base, "/") + "/" + path
}

func exportResolvePathFromURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	path := u.Path
	if u.RawQuery != "" {
		path += "?" + u.RawQuery
	}
	return path
}

func exportInferConversationMode(rawModel string, inferred *InferredIntegration) string {
	if inferred != nil && inferred.Profile != nil {
		mode := strings.TrimSpace(inferred.Profile.Conversation.Mode)
		if mode != "" {
			return exportNormalizeConversationMode(mode)
		}
	}
	return exportNormalizeConversationMode(rawModel)
}

func exportNormalizeConversationMode(mode string) string {
	mode = strings.TrimSpace(mode)
	if mode == "" {
		return "remote_session"
	}
	return mode
}

func exportCloneMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = exportCloneValue(v)
	}
	return out
}

func exportCloneValue(v any) any {
	switch val := v.(type) {
	case map[string]any:
		return exportCloneMap(val)
	case []any:
		out := make([]any, len(val))
		for i, item := range val {
			out[i] = exportCloneValue(item)
		}
		return out
	default:
		return v
	}
}
