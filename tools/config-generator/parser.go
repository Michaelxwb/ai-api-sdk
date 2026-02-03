package generator

import (
	"encoding/json"
	"errors"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

type PlatformInfo struct {
	Name         string
	ProviderName string
	BaseURL      string
	Path         string
	Headers      map[string]string
	RequestBody  map[string]any
	ResponseBody map[string]any
	RequestRaw   string
	ResponseRaw  string
	DefaultModel string
	ExtraBody    map[string]any
	QueryParams  map[string]string
	IsStream     bool
}

var headingRe = regexp.MustCompile(`^##\s*`)

// ParsePlatforms parses the request/response markdown and returns platforms.
func ParsePlatforms(input string) ([]PlatformInfo, error) {
	clean := strings.ReplaceAll(input, "\r\n", "\n")
	lines := strings.Split(clean, "\n")

	var platforms []PlatformInfo
	var current *PlatformInfo
	var inCode bool
	var blockLines []string
	blockCount := 0

	flushBlock := func() {
		if current == nil {
			return
		}
		raw := strings.TrimSpace(strings.Join(blockLines, "\n"))
		if raw == "" {
			return
		}
		switch blockCount {
		case 0:
			current.RequestRaw = raw
		case 1:
			current.ResponseRaw = raw
		}
		blockCount++
	}

	flushPlatform := func() {
		if current == nil {
			return
		}
		if current.Name != "" {
			platforms = append(platforms, *current)
		}
		current = nil
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "##") {
			if inCode {
				flushBlock()
				inCode = false
				blockLines = nil
			}
			flushPlatform()
			name := parsePlatformName(trimmed)
			current = &PlatformInfo{Name: name}
			blockCount = 0
			continue
		}
		if current == nil {
			continue
		}
		if strings.HasPrefix(trimmed, "URL") {
			urlStr := extractURL(trimmed)
			if urlStr != "" {
				baseURL, pathValue, query := parseURL(urlStr)
				if baseURL != "" {
					current.BaseURL = baseURL
				}
				if pathValue != "" {
					current.Path = pathValue
				}
				if len(query) > 0 {
					current.QueryParams = query
				}
			}
			continue
		}
		if strings.HasPrefix(trimmed, "```") {
			if inCode {
				flushBlock()
				inCode = false
				blockLines = nil
			} else {
				inCode = true
				blockLines = nil
			}
			continue
		}
		if inCode {
			blockLines = append(blockLines, line)
		}
	}
	if inCode {
		flushBlock()
	}
	flushPlatform()

	if len(platforms) == 0 {
		return nil, errors.New("未找到任何平台段落")
	}
	for i := range platforms {
		if err := finalizePlatform(&platforms[i]); err != nil {
			return nil, err
		}
	}
	return platforms, nil
}

func parsePlatformName(line string) string {
	name := headingRe.ReplaceAllString(line, "")
	name = strings.TrimSpace(name)
	name = strings.TrimLeftFunc(name, func(r rune) bool {
		return unicode.IsDigit(r) || r == '、' || r == '.' || r == ')' || r == '(' || unicode.IsSpace(r)
	})
	name = strings.TrimSpace(name)
	name = strings.TrimSuffix(name, "平台")
	return strings.TrimSpace(name)
}

func extractURL(line string) string {
	idx := strings.Index(line, "http")
	if idx == -1 {
		return ""
	}
	return strings.TrimSpace(line[idx:])
}

func parseURL(urlStr string) (string, string, map[string]string) {
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return "", "", nil
	}
	baseURL := ""
	if parsed.Scheme != "" && parsed.Host != "" {
		baseURL = parsed.Scheme + "://" + parsed.Host
	}
	pathValue := parsed.Path
	query := parseQueryValues(parsed.Query())
	return baseURL, pathValue, query
}

func finalizePlatform(p *PlatformInfo) error {
	if p == nil {
		return nil
	}
	if p.RequestRaw == "" {
		return errors.New("缺少请求段落")
	}
	req, err := parseRequestBlock(p.RequestRaw)
	if err != nil {
		return err
	}
	p.Headers = req.Headers
	p.RequestBody = req.Body
	if req.Path != "" {
		p.Path = req.Path
	}
	if strings.Contains(p.Path, "?") {
		if parsed, err := url.Parse(p.Path); err == nil {
			if q := parseQueryValues(parsed.Query()); len(q) > 0 {
				if p.QueryParams == nil {
					p.QueryParams = map[string]string{}
				}
				for k, v := range q {
					p.QueryParams[k] = v
				}
			}
			if parsed.Path != "" {
				p.Path = parsed.Path
			}
		}
	}
	if len(req.QueryParams) > 0 {
		if p.QueryParams == nil {
			p.QueryParams = map[string]string{}
		}
		for k, v := range req.QueryParams {
			p.QueryParams[k] = v
		}
	}
	if p.Path == "" {
		p.Path = "/chat/completions"
	}
	if p.BaseURL == "" {
		if host := headerValue(req.Headers, "Host"); host != "" {
			p.BaseURL = "https://" + host
		}
	}
	if p.BaseURL != "" {
		p.BaseURL = strings.TrimRight(p.BaseURL, "/")
	}
	if p.Path != "" && !strings.HasPrefix(p.Path, "/") {
		p.Path = "/" + p.Path
	}
	if p.ProviderName == "" {
		host := strings.TrimPrefix(p.BaseURL, "https://")
		host = strings.TrimPrefix(host, "http://")
		p.ProviderName = deriveProviderName(host, p.Name)
	}
	if p.Name == "" && p.ProviderName != "" {
		p.Name = p.ProviderName
	}
	if p.RequestBody != nil {
		if model, ok := p.RequestBody["model"].(string); ok {
			p.DefaultModel = model
		}
		p.ExtraBody = detectExtraBody(p.RequestBody)
	}
	if p.ResponseRaw != "" {
		resp, _ := parseResponseBlock(p.ResponseRaw)
		p.ResponseBody = resp.Body
		p.IsStream = resp.IsStream
	}
	return nil
}

type requestBlock struct {
	Method      string
	Path        string
	Headers     map[string]string
	Body        map[string]any
	QueryParams map[string]string
}

func parseRequestBlock(raw string) (requestBlock, error) {
	lines := strings.Split(raw, "\n")
	if len(lines) == 0 {
		return requestBlock{}, errors.New("请求块为空")
	}
	first := strings.TrimSpace(lines[0])
	parts := strings.Fields(first)
	method := ""
	pathValue := ""
	queryParams := map[string]string{}
	if len(parts) >= 2 {
		method = parts[0]
		pathValue = parts[1]
	}
	if pathValue != "" {
		pathValue = strings.TrimSpace(pathValue)
		if parsed, err := url.Parse(pathValue); err == nil {
			if parsed.Path != "" {
				pathValue = parsed.Path
			}
			if q := parseQueryValues(parsed.Query()); len(q) > 0 {
				queryParams = q
			}
		}
	}

	headers := map[string]string{}
	var bodyLines []string
	readingBody := false
	for i := 1; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], "\r")
		if !readingBody {
			if strings.TrimSpace(line) == "" {
				readingBody = true
				continue
			}
			if looksLikeBodyStart(line) {
				readingBody = true
				bodyLines = append(bodyLines, line)
				continue
			}
			if k, v, ok := splitHeader(line); ok {
				headers[k] = v
			}
			continue
		}
		bodyLines = append(bodyLines, line)
	}
	bodyRaw := strings.TrimSpace(strings.Join(bodyLines, "\n"))
	body := map[string]any{}
	if bodyRaw != "" && (strings.HasPrefix(bodyRaw, "{") || strings.HasPrefix(bodyRaw, "[")) {
		var data any
		if err := json.Unmarshal([]byte(bodyRaw), &data); err == nil {
			if obj, ok := data.(map[string]any); ok {
				body = obj
			}
		}
	}
	if len(queryParams) == 0 {
		queryParams = nil
	}
	return requestBlock{Method: method, Path: pathValue, Headers: headers, Body: body, QueryParams: queryParams}, nil
}

type responseBlock struct {
	Headers  map[string]string
	Body     map[string]any
	IsStream bool
}

func parseResponseBlock(raw string) (responseBlock, error) {
	lines := strings.Split(raw, "\n")
	if len(lines) == 0 {
		return responseBlock{}, errors.New("响应块为空")
	}
	headers := map[string]string{}
	var bodyLines []string
	readingBody := false
	for i := 1; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], "\r")
		if !readingBody {
			if strings.TrimSpace(line) == "" {
				readingBody = true
				continue
			}
			if looksLikeBodyStart(line) {
				readingBody = true
				bodyLines = append(bodyLines, line)
				continue
			}
			if k, v, ok := splitHeader(line); ok {
				headers[k] = v
			}
			continue
		}
		bodyLines = append(bodyLines, line)
	}
	bodyRaw := strings.TrimSpace(strings.Join(bodyLines, "\n"))
	out := responseBlock{Headers: headers}
	if bodyRaw == "" {
		return out, nil
	}
	for _, line := range strings.Split(bodyRaw, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "data:") {
			out.IsStream = true
			payload := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
			var data any
			if err := json.Unmarshal([]byte(payload), &data); err == nil {
				if obj, ok := data.(map[string]any); ok {
					out.Body = obj
					return out, nil
				}
			}
		}
	}
	if strings.HasPrefix(bodyRaw, "{") || strings.HasPrefix(bodyRaw, "[") {
		var data any
		if err := json.Unmarshal([]byte(bodyRaw), &data); err == nil {
			if obj, ok := data.(map[string]any); ok {
				out.Body = obj
				return out, nil
			}
		}
	}
	return out, nil
}

func parseQueryValues(values url.Values) map[string]string {
	if len(values) == 0 {
		return nil
	}
	query := map[string]string{}
	for k, v := range values {
		if len(v) > 0 {
			query[k] = v[0]
		}
	}
	if len(query) == 0 {
		return nil
	}
	return query
}

func splitHeader(line string) (string, string, bool) {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])
	if key == "" {
		return "", "", false
	}
	return key, value, true
}

func looksLikeBodyStart(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[")
}

func headerValue(headers map[string]string, key string) string {
	if headers == nil {
		return ""
	}
	if v, ok := headers[key]; ok {
		return v
	}
	for k, v := range headers {
		if strings.EqualFold(k, key) {
			return v
		}
	}
	return ""
}

func deriveProviderName(host, fallback string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return sanitizeName(fallback)
	}
	host = strings.ToLower(host)
	host = strings.Split(host, ":")[0]
	labels := strings.Split(host, ".")
	if len(labels) > 1 {
		if labels[0] == "api" || labels[0] == "www" {
			return sanitizeName(labels[1])
		}
		return sanitizeName(labels[0])
	}
	return sanitizeName(host)
}

func sanitizeName(name string) string {
	if name == "" {
		return "provider"
	}
	var b strings.Builder
	lastUnderscore := false
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "provider"
	}
	return out
}

func detectExtraBody(requestBody map[string]any) map[string]any {
	if len(requestBody) == 0 {
		return nil
	}
	standardFields := map[string]struct{}{
		"model": {}, "messages": {}, "temperature": {}, "max_tokens": {}, "stream": {}, "top_p": {}, "frequency_penalty": {}, "presence_penalty": {},
	}
	extra := map[string]any{}
	for key, value := range requestBody {
		if _, ok := standardFields[key]; ok {
			continue
		}
		extra[key] = value
	}
	if len(extra) == 0 {
		return nil
	}
	return extra
}

func sortedKeys(m map[string]string) []string {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedAnyKeys(m map[string]any) []string {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func normalizePath(baseURL, p string) string {
	if p == "" {
		return ""
	}
	if strings.HasPrefix(p, "http") {
		parsed, err := url.Parse(p)
		if err == nil {
			return parsed.Path
		}
	}
	clean := path.Clean(p)
	if !strings.HasPrefix(clean, "/") {
		clean = "/" + clean
	}
	return clean
}
