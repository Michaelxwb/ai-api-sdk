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

// RawHTTPMultiRoundSpec 是业务层提供的多轮抓包原文格式。
// SDK 内部会将每轮 request/response 原文自动转换为 MultiRoundSpec 以进行推理。
type RawHTTPMultiRoundSpec struct {
	// Model 是会话模式：remote_session（服务端维护会话）或 local_history（本地维护历史）
	Model string `json:"model"`
	// BaseURL 是目标接口完整 URL（含 scheme + host + path）
	BaseURL string `json:"base_url"`
	// Rounds 需提供 2~5 轮完整请求/响应原文。
	Rounds []RawHTTPRound `json:"rounds"`
}

// RawHTTPRound 是一轮抓包原文（request/response）。
type RawHTTPRound struct {
	Request  string `json:"request"`
	Response string `json:"response"`
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

// ParseHTTPMultiRoundSpec 将业务层抓包多轮原文转换为推理层 MultiRoundSpec。
// 请求体会被规范化为紧凑 JSON 字符串，响应体会提取为可被推理器消费的 JSON 对象字符串。
func ParseHTTPMultiRoundSpec(spec RawHTTPMultiRoundSpec) (MultiRoundSpec, error) {
	if strings.TrimSpace(spec.BaseURL) == "" {
		return MultiRoundSpec{}, fmt.Errorf("generic: base_url is required")
	}
	if len(spec.Rounds) == 0 {
		return MultiRoundSpec{}, fmt.Errorf("generic: rounds is required")
	}
	if len(spec.Rounds) < 2 || len(spec.Rounds) > 5 {
		return MultiRoundSpec{}, fmt.Errorf("generic: rounds must contain 2-5 items")
	}

	mode := strings.TrimSpace(spec.Model)
	if mode == "" {
		return MultiRoundSpec{}, fmt.Errorf("generic: model is required")
	}
	if mode != "remote_session" && mode != "local_history" {
		return MultiRoundSpec{}, fmt.Errorf("generic: invalid model %q, must be remote_session or local_history", spec.Model)
	}

	rounds := make([]RoundPair, len(spec.Rounds))
	for i, r := range spec.Rounds {
		if strings.TrimSpace(r.Request) == "" {
			return MultiRoundSpec{}, fmt.Errorf("generic: rounds[%d].request is required", i)
		}
		if strings.TrimSpace(r.Response) == "" {
			return MultiRoundSpec{}, fmt.Errorf("generic: rounds[%d].response is required", i)
		}

		_, reqHeaders, reqBodyMap, err := parseHTTPRequest(r.Request)
		if err != nil {
			return MultiRoundSpec{}, fmt.Errorf("generic: parse rounds[%d].request failed: %w", i, err)
		}
		if reqBodyMap == nil {
			return MultiRoundSpec{}, fmt.Errorf("generic: rounds[%d].request body is required", i)
		}
		reqBodyBytes, err := json.Marshal(reqBodyMap)
		if err != nil {
			return MultiRoundSpec{}, fmt.Errorf("generic: marshal rounds[%d].request body failed: %w", i, err)
		}

		respHeaders, respBody, err := extractHTTPResponseJSONBodyForInference(r.Response)
		if err != nil {
			return MultiRoundSpec{}, fmt.Errorf("generic: parse rounds[%d].response failed: %w", i, err)
		}

		rounds[i] = RoundPair{
			Request: RawPacket{
				Headers: reqHeaders,
				Body:    string(reqBodyBytes),
			},
			Response: RawPacket{
				Headers: respHeaders,
				Body:    respBody,
			},
		}
	}

	return MultiRoundSpec{
		URL:          spec.BaseURL,
		Rounds:       rounds,
		Conversation: ConversationProfile{Mode: mode},
	}, nil
}

// extractHTTPResponseJSONBodyForInference 从完整 HTTP/SSE 响应提取可用于推理的 JSON 对象字符串。
// - 普通 JSON 响应：返回 trim 后 body 原文（必须是 JSON 对象）
// - SSE 响应：按优先级选择 event:complete > data.status=="complete" > 最后一个可解析 JSON 对象帧
func extractHTTPResponseJSONBodyForInference(text string) (map[string]string, string, error) {
	if _, err := parseHTTPResponseSpec(text, nil); err != nil {
		return nil, "", err
	}

	respHeaders, bodyLines, isSSE := splitHTTPResponseText(text)
	respBody := strings.TrimSpace(strings.Join(bodyLines, "\n"))
	if respBody == "" {
		return nil, "", fmt.Errorf("response body is required")
	}

	if !isSSE {
		var obj map[string]any
		if err := json.Unmarshal([]byte(respBody), &obj); err != nil {
			return nil, "", fmt.Errorf("response body is not valid JSON object: %w", err)
		}
		return respHeaders, respBody, nil
	}

	rawFrames, _ := parseRawSSEFrames(bodyLines)
	var eventCompleteObj map[string]any
	var statusCompleteObj map[string]any
	var lastJSONObj map[string]any

	for _, f := range rawFrames {
		var obj map[string]any
		if err := json.Unmarshal([]byte(f.data), &obj); err != nil {
			continue
		}
		lastJSONObj = obj
		if strings.EqualFold(strings.TrimSpace(f.event), "complete") {
			eventCompleteObj = obj
		}
		if status, ok := obj["status"].(string); ok && strings.EqualFold(strings.TrimSpace(status), "complete") {
			statusCompleteObj = obj
		}
	}

	selected := eventCompleteObj
	if selected == nil {
		selected = statusCompleteObj
	}
	if selected == nil {
		selected = lastJSONObj
	}
	if selected == nil {
		return nil, "", fmt.Errorf("sse response has no parseable JSON object data frame")
	}

	normalized, err := json.Marshal(selected)
	if err != nil {
		return nil, "", fmt.Errorf("marshal selected SSE frame failed: %w", err)
	}
	return respHeaders, string(normalized), nil
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
				// 跳过传输层 headers：
				// - Host: 从 base_url 推导
				// - Accept-Encoding: 由 Go HTTP 自动管理，保留会导致服务端返回 brotli/zstd 等 SDK 无法解压的编码
				// - Content-Length: 由 Go 根据 body 自动计算
				// - Connection / Transfer-Encoding: HTTP/2 自动管理
				lk := strings.ToLower(k)
				if lk == "host" || lk == "accept-encoding" || lk == "content-length" ||
					lk == "connection" || lk == "transfer-encoding" {
					continue
				}
				headers[k] = v
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

// rawSSEFrame 表示一条 SSE data 行及其 event 上下文。
type rawSSEFrame struct {
	event string
	data  string
}

// splitHTTPResponseText 解析原始 HTTP 响应文本，返回 headers、bodyLines 和是否为 SSE。
// 支持仅提供 SSE body（无 HTTP 响应头）的输入。
func splitHTTPResponseText(text string) (map[string]string, []string, bool) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.Split(text, "\n")

	startsWithSSE := false
	startsWithBody := false
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "event:") || strings.HasPrefix(trimmed, "data:") {
			startsWithSSE = true
		}
		if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
			startsWithBody = true
		}
		break
	}

	headers := make(map[string]string)
	inBody := startsWithSSE || startsWithBody // 若以 body 内容开头，跳过 HTTP 头解析
	isSSE := startsWithSSE
	var bodyLines []string

	for _, line := range lines {
		if !inBody {
			lower := strings.ToLower(line)
			if strings.HasPrefix(lower, "content-type:") && strings.Contains(lower, "text/event-stream") {
				isSSE = true
			}
			if strings.TrimSpace(line) == "" {
				inBody = true
				continue
			}
			if idx := strings.Index(line, ":"); idx > 0 {
				k := strings.TrimSpace(line[:idx])
				v := strings.TrimSpace(line[idx+1:])
				headers[k] = v
			}
			continue
		}
		bodyLines = append(bodyLines, line)
	}

	// 若响应头无 SSE 标识，从 body 自动检测。
	if !isSSE {
		for _, line := range bodyLines {
			if strings.HasPrefix(strings.TrimSpace(line), "data:") {
				isSSE = true
				break
			}
		}
	}

	return headers, bodyLines, isSSE
}

// parseHTTPResponseSpec 从原始 HTTP 响应文本自动推断流式响应配置。
func parseHTTPResponseSpec(text string, chainFields []ChainField) (RawResponseSpec, error) {
	if strings.TrimSpace(text) == "" {
		// 响应文本为空时返回空配置，由调用方按需补充
		return RawResponseSpec{}, nil
	}

	_, bodyLines, isSSE := splitHTTPResponseText(text)

	protocol := "ndjson"
	if isSSE {
		protocol = "sse"
	}

	// 非 SSE 场景：若 body 是单个完整 JSON 对象（普通 application/json 响应），
	// 直接按 "json" 协议解析，从对象中收集占位符路径，跳过按行 SSE 帧扫描。
	if !isSSE {
		if spec, ok := tryBuildJSONResponseSpec(bodyLines, chainFields); ok {
			return spec, nil
		}
	}

	// 解析 SSE 帧，同时跟踪 SSE 协议级 event: 头。
	// 支持两种 data 格式：JSON 对象（Dify 风格）和原始 JSON 字符串（OpenAI 风格）。
	rawFrames, hasDoneMarker := parseRawSSEFrames(bodyLines)

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
			} else if f.event != "" && isTerminalEventName(f.event) && sseEventDone == "" {
				// 非字符串类型（如 JSON boolean "true"）：仅通过 event 名称判断终止
				sseEventDone = f.event
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

// parseRawSSEFrames 解析 SSE body 行，返回 data 帧及 [DONE] 标记。
func parseRawSSEFrames(bodyLines []string) ([]rawSSEFrame, bool) {
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
			data := normalizeRawSSEDataValue(strings.TrimSpace(trimmed[5:]))
			rawFrames = append(rawFrames, rawSSEFrame{event: curEvent, data: data})
		}
	}

	return rawFrames, hasDoneMarker
}

func normalizeRawSSEDataValue(value string) string {
	trimmed := strings.TrimSpace(value)
	if !strings.HasPrefix(trimmed, "data:") {
		return value
	}

	nested := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
	if nested == "" || nested == "[DONE]" || isLikelyRawJSONValue(nested) {
		return nested
	}
	return value
}

func isLikelyRawJSONValue(value string) bool {
	if value == "" {
		return false
	}
	switch value[0] {
	case '{', '[', '"':
		return true
	}
	return value == "true" || value == "false" || value == "null" ||
		(value[0] >= '0' && value[0] <= '9') || value[0] == '-'
}

// isTerminalEventName 判断 SSE 事件名称是否表示流结束。
func isTerminalEventName(name string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(lower, "end") || strings.Contains(lower, "done") ||
		strings.Contains(lower, "finish") || strings.Contains(lower, "stop") ||
		strings.Contains(lower, "complete") || strings.Contains(lower, "close")
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
// 当占位符位于数组最后一个元素时，使用 -1（最后一个元素）代替硬编码索引，
// 因为模板中的数组长度通常只是示例，实际响应中数组大小可能不同。
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
			idxStr := fmt.Sprintf("%d", i)
			if i == len(val)-1 {
				idxStr = "-1" // 最后一个元素 → 用 -1 适配变长数组
			}
			var path string
			if prefix != "" {
				path = prefix + "." + idxStr
			} else {
				path = idxStr
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
// 同 collectPlaceholderPaths，数组最后一个元素使用 -1 索引。
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
			idxStr := fmt.Sprintf("%d", i)
			if i == len(val)-1 {
				idxStr = "-1"
			}
			var path string
			if prefix != "" {
				path = prefix + "." + idxStr
			} else {
				path = idxStr
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

// tryBuildJSONResponseSpec 尝试把 bodyLines 作为单个 JSON 对象解析。
// 成功则返回 "json" 协议的 RawResponseSpec，ok=true；否则 ok=false，由调用方走回退逻辑。
func tryBuildJSONResponseSpec(bodyLines []string, chainFields []ChainField) (RawResponseSpec, bool) {
	bodyText := strings.TrimSpace(strings.Join(bodyLines, "\n"))
	if bodyText == "" {
		return RawResponseSpec{}, false
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(bodyText), &obj); err != nil {
		return RawResponseSpec{}, false
	}

	seen := make(map[string]struct{})
	var deltaPaths []string
	collectPlaceholderPaths(obj, "", seen, &deltaPaths)
	sort.Strings(deltaPaths)

	var textPath string
	if len(deltaPaths) > 0 {
		textPath = deltaPaths[0]
	}

	remoteIDPath := findFirstPlaceholder(obj, "", "$$$SESSION_ID$$$")

	return RawResponseSpec{
		TextPath:     textPath,
		RemoteIDPath: remoteIDPath,
		Stream: StreamProfile{
			Protocol:    "json",
			DeltaPaths:  deltaPaths,
			ChainFields: chainFields,
		},
	}, true
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
