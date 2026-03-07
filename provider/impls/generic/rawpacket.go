package generic

import (
	"bufio"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// RawHTTPSpec 是业务层提供的五字段接入格式。
// 业务层无需了解 SDK 内部的 RawIntegrationSpec 结构，
// 只需提供从浏览器/抓包工具获取的原始 HTTP 报文即可。
type RawHTTPSpec struct {
	// Model 是会话模式：remote_session（服务端维护会话）或 local_history（本地维护历史）
	Model string `json:"model"`
	// BaseURL 是目标接口完整 URL（含 scheme + host + path）
	BaseURL string `json:"base_url"`
	// Request 是原始 HTTP 请求文本。
	// "$$$" 为用户输入占位符，"$$$SESSION_ID$$$" 为会话 ID 占位符，
	// "$$$NAME$$$" 形式为链路字段占位符（如 "$$$PARENT_MSG$$$"）。
	Request string `json:"request"`
	// Response 是代表性 HTTP 响应文本，用于自动推断流式协议、delta 字段、结束条件等。
	// SSE 场景应包含至少两个含 delta 内容的数据帧，以便 SDK 自动识别 delta 路径。
	Response string `json:"response"`
	// ChainFields 描述多轮字段链路传递规则，可不传。
	// 传了则每条的 Placeholder、ResponsePath、ExtractOnEvent 均必填。
	ChainFields []ChainField `json:"chain_fields"`
}

// ParseHTTPSpec 将业务层五字段格式解析为 RawIntegrationSpec，
// 可直接传入 ParseRawIntegration 进一步编译为 GenericProfile。
func ParseHTTPSpec(spec RawHTTPSpec) (RawIntegrationSpec, error) {
	if strings.TrimSpace(spec.BaseURL) == "" {
		return RawIntegrationSpec{}, fmt.Errorf("generic: base_url is required")
	}
	if strings.TrimSpace(spec.Request) == "" {
		return RawIntegrationSpec{}, fmt.Errorf("generic: request is required")
	}

	method, headers, body, err := parseHTTPRequest(spec.Request)
	if err != nil {
		return RawIntegrationSpec{}, fmt.Errorf("generic: parse request failed: %w", err)
	}

	respSpec, err := parseHTTPResponseSpec(spec.Response, spec.ChainFields)
	if err != nil {
		return RawIntegrationSpec{}, fmt.Errorf("generic: parse response failed: %w", err)
	}

	return RawIntegrationSpec{
		URL:          spec.BaseURL,
		Method:       method,
		Headers:      headers,
		Body:         body,
		Response:     respSpec,
		Conversation: ConversationProfile{Mode: spec.Model},
		ChainFields:  spec.ChainFields,
	}, nil
}

// parseHTTPRequest 从原始 HTTP 请求文本解析出 method、headers 和 body。
func parseHTTPRequest(text string) (method string, headers map[string]string, body map[string]any, err error) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	scanner := bufio.NewScanner(strings.NewReader(text))

	// 找到第一个有效行（跳过注释和空行）
	var firstLine string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		firstLine = line
		break
	}
	if firstLine == "" {
		return "", nil, nil, fmt.Errorf("request text is empty or has no method line")
	}

	// 第一个 token 即为 HTTP method
	parts := strings.Fields(firstLine)
	method = strings.ToUpper(parts[0])

	// 解析 headers（直到空行），body 在空行之后
	headers = make(map[string]string)
	var bodyLines []string
	inBody := false

	for scanner.Scan() {
		raw := scanner.Text()
		trimmed := strings.TrimSpace(raw)

		if !inBody {
			if trimmed == "" {
				inBody = true
				continue
			}
			if strings.HasPrefix(trimmed, "#") {
				continue
			}
			idx := strings.Index(raw, ":")
			if idx > 0 {
				k := strings.TrimSpace(raw[:idx])
				v := strings.TrimSpace(raw[idx+1:])
				// 跳过 Host（从 base_url 推导，无需重复传入）
				if !strings.EqualFold(k, "host") {
					headers[k] = v
				}
			}
		} else {
			if !strings.HasPrefix(trimmed, "#") {
				bodyLines = append(bodyLines, raw)
			}
		}
	}

	bodyText := strings.TrimSpace(strings.Join(bodyLines, "\n"))
	if bodyText == "" {
		return method, headers, nil, nil
	}

	var bodyMap map[string]any
	if jsonErr := json.Unmarshal([]byte(bodyText), &bodyMap); jsonErr != nil {
		return "", nil, nil, fmt.Errorf("request body is not valid JSON: %w", jsonErr)
	}

	return method, headers, bodyMap, nil
}

// hexOnlyRe 匹配 HTTP chunked 传输编码的块大小行（纯十六进制数字）。
var hexOnlyRe = regexp.MustCompile(`^[0-9a-fA-F]+$`)

// parseHTTPResponseSpec 从原始 HTTP 响应文本自动推断流式响应配置。
func parseHTTPResponseSpec(text string, chainFields []ChainField) (RawResponseSpec, error) {
	if strings.TrimSpace(text) == "" {
		// 响应文本为空时返回空配置，由调用方按需补充
		return RawResponseSpec{}, nil
	}

	text = strings.ReplaceAll(text, "\r\n", "\n")

	// 分离响应头与响应体
	inBody := false
	isSSE := false
	var bodyLines []string

	for _, line := range strings.Split(text, "\n") {
		if !inBody {
			lower := strings.ToLower(line)
			if strings.HasPrefix(lower, "content-type:") {
				if strings.Contains(lower, "text/event-stream") {
					isSSE = true
				}
			}
			if strings.TrimSpace(line) == "" {
				inBody = true
			}
		} else {
			bodyLines = append(bodyLines, line)
		}
	}

	// 若响应头无 SSE 标识，从 body 自动检测
	if !isSSE {
		for _, line := range bodyLines {
			if strings.HasPrefix(strings.TrimSpace(line), "data:") {
				isSSE = true
				break
			}
		}
	}

	protocol := "ndjson"
	if isSSE {
		protocol = "sse"
	}

	// 解析 SSE 帧，同时跟踪 SSE 协议级 event: 头
	// 支持两种 data 格式：JSON 对象（Dify 风格）和原始 JSON 字符串（OpenAI 风格）
	type rawSSEFrame struct {
		event string
		data  string
	}
	var rawFrames []rawSSEFrame
	var curEvent string
	hasDoneMarker := false

	for _, line := range bodyLines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			curEvent = "" // SSE 帧分隔符，重置当前事件类型
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		// 跳过 HTTP chunked 编码块大小行
		if hexOnlyRe.MatchString(trimmed) {
			continue
		}
		// 跳过 SSE id/retry 字段
		if strings.HasPrefix(trimmed, "id:") || strings.HasPrefix(trimmed, "retry:") {
			continue
		}
		if strings.HasPrefix(trimmed, "event:") {
			curEvent = strings.TrimSpace(trimmed[6:])
			continue
		}
		if trimmed == "data: [DONE]" || trimmed == "data:[DONE]" {
			hasDoneMarker = true
			curEvent = ""
			continue
		}
		if strings.HasPrefix(trimmed, "data:") {
			data := strings.TrimSpace(trimmed[5:])
			rawFrames = append(rawFrames, rawSSEFrame{event: curEvent, data: data})
		}
	}

	// 分析各帧：区分 JSON 对象帧（Dify 风格）和原始字符串帧（OpenAI 风格）
	var jsonFrames []map[string]any
	rawDataIsDelta := false
	var sseEventDone string

	for _, f := range rawFrames {
		var obj map[string]any
		if jsonErr := json.Unmarshal([]byte(f.data), &obj); jsonErr == nil {
			// JSON 对象帧：若 SSE 协议级 event 存在且帧内无 event 字段，则注入
			if f.event != "" {
				if _, ok := obj["event"]; !ok {
					obj["event"] = f.event
				}
			}
			jsonFrames = append(jsonFrames, obj)
		} else {
			// 非 JSON 对象：尝试解析为原始字符串
			var s string
			if jsonErr2 := json.Unmarshal([]byte(f.data), &s); jsonErr2 == nil {
				if s == "$$$" {
					// 占位符：标记 data 行本身即为 delta 内容（空路径模式）
					rawDataIsDelta = true
				} else if f.event != "" && isTerminalEventName(f.event) && sseEventDone == "" {
					// SSE 协议级终止事件
					sseEventDone = f.event
				}
			}
		}
	}

	// 推断 delta 路径和远端会话 ID
	var deltaPaths []string
	var remoteIDPath string
	if rawDataIsDelta {
		// 空路径模式：MakeJSONPathMultiExtractor 将直接把 data 值解码为字符串
		deltaPaths = []string{""}
	} else {
		deltaPaths = detectDeltaPathsByPlaceholder(jsonFrames)
		remoteIDPath = detectRemoteIDByPlaceholder(jsonFrames)
	}

	// 推断结束条件
	var donePath, doneValue, doneMarker string
	if hasDoneMarker {
		doneMarker = "[DONE]"
	}
	// 优先级 1：ChainFields 中的 ExtractOnEvent（donePath = "event" 比对 JSON 嵌入事件）
	for _, cf := range chainFields {
		if cf.ExtractOnEvent != "" {
			donePath = "event"
			doneValue = cf.ExtractOnEvent
			break
		}
	}
	// 优先级 2：SSE 协议级终止事件（donePath = "" 表示比对 SSE event 类型参数）
	if doneValue == "" && sseEventDone != "" {
		donePath = ""
		doneValue = sseEventDone
	}
	// 优先级 3：从 JSON 对象帧中检测终止事件（donePath = "event" 比对 JSON 嵌入事件）
	if doneValue == "" {
		if termEvent := detectTerminalEvent(jsonFrames); termEvent != "" {
			donePath = "event"
			doneValue = termEvent
		}
	}

	// 非流式 TextPath 取 delta 路径的第一个（空路径表示无固定字段，不适用）
	var textPath string
	if len(deltaPaths) > 0 && deltaPaths[0] != "" {
		textPath = deltaPaths[0]
	}

	return RawResponseSpec{
		TextPath:     textPath,
		RemoteIDPath: remoteIDPath,
		Stream: StreamProfile{
			Protocol:   protocol,
			DeltaPaths: deltaPaths,
			DonePath:   donePath,
			DoneValue:  doneValue,
			DoneMarker: doneMarker,
		},
	}, nil
}

// isTerminalEventName 判断 SSE 事件名称是否表示流结束。
func isTerminalEventName(name string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(lower, "end") || strings.Contains(lower, "done") ||
		strings.Contains(lower, "finish") || strings.Contains(lower, "stop") ||
		strings.Contains(lower, "complete")
}

// detectDeltaPathsByPlaceholder 从响应帧中找出值为 "$$$" 的字段路径，作为流式 delta 内容路径。
// 支持嵌套路径（如 choices.0.delta.content），业务层只需将答案字段值写成 "$$$" 即可。
func detectDeltaPathsByPlaceholder(frames []map[string]any) []string {
	seen := make(map[string]struct{})
	var deltaPaths []string
	for _, frame := range frames {
		collectPlaceholderPaths(frame, "", seen, &deltaPaths)
	}
	sort.Strings(deltaPaths)
	return deltaPaths
}

// collectPlaceholderPaths 递归遍历 JSON 值，收集所有值为 "$$$" 的字段路径（点分隔）。
func collectPlaceholderPaths(v any, prefix string, seen map[string]struct{}, paths *[]string) {
	switch val := v.(type) {
	case map[string]any:
		for k, child := range val {
			path := k
			if prefix != "" {
				path = prefix + "." + k
			}
			collectPlaceholderPaths(child, path, seen, paths)
		}
	case []any:
		for i, child := range val {
			var path string
			if prefix != "" {
				path = fmt.Sprintf("%s.%d", prefix, i)
			} else {
				path = fmt.Sprintf("%d", i)
			}
			collectPlaceholderPaths(child, path, seen, paths)
		}
	case string:
		if val == "$$$" {
			if _, dup := seen[prefix]; !dup {
				seen[prefix] = struct{}{}
				*paths = append(*paths, prefix)
			}
		}
	}
}

// detectRemoteIDByPlaceholder 从响应帧中找出值为 "$$$SESSION_ID$$$" 的字段路径，作为远端会话 ID 路径。
func detectRemoteIDByPlaceholder(frames []map[string]any) string {
	for _, frame := range frames {
		if path := findFirstPlaceholder(frame, "", "$$$SESSION_ID$$$"); path != "" {
			return path
		}
	}
	return ""
}

// findFirstPlaceholder 递归遍历 JSON 值，返回第一个匹配目标占位符的字段路径。
func findFirstPlaceholder(v any, prefix, target string) string {
	switch val := v.(type) {
	case map[string]any:
		for k, child := range val {
			path := k
			if prefix != "" {
				path = prefix + "." + k
			}
			if found := findFirstPlaceholder(child, path, target); found != "" {
				return found
			}
		}
	case []any:
		for i, child := range val {
			var path string
			if prefix != "" {
				path = fmt.Sprintf("%s.%d", prefix, i)
			} else {
				path = fmt.Sprintf("%d", i)
			}
			if found := findFirstPlaceholder(child, path, target); found != "" {
				return found
			}
		}
	case string:
		if val == target {
			return prefix
		}
	}
	return ""
}

// detectTerminalEvent 从响应帧中推断终止事件类型。
// 判定条件：只出现一次且名称含 end/done/finish/stop/complete 关键字。
func detectTerminalEvent(frames []map[string]any) string {
	eventCount := make(map[string]int)
	for _, f := range frames {
		if v, ok := f["event"].(string); ok && v != "" {
			eventCount[v]++
		}
	}
	for ev, cnt := range eventCount {
		if cnt == 1 {
			lower := strings.ToLower(ev)
			if strings.Contains(lower, "end") || strings.Contains(lower, "done") ||
				strings.Contains(lower, "finish") || strings.Contains(lower, "stop") ||
				strings.Contains(lower, "complete") {
				return ev
			}
		}
	}
	return ""
}
