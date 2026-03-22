package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Michaelxwb/ai-api-sdk/client"
	"github.com/Michaelxwb/ai-api-sdk/examples/sessionstore"
	"github.com/Michaelxwb/ai-api-sdk/provider/base"
	"github.com/Michaelxwb/ai-api-sdk/provider/impls/generic"

	_ "github.com/Michaelxwb/ai-api-sdk/provider"
)

// debugTransport prints the request body before forwarding.
type debugTransport struct {
	base http.RoundTripper
}

func (d *debugTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		body, _ := io.ReadAll(req.Body)
		fmt.Printf("[DEBUG] 请求体: %s\n", string(body))
		req.Body = io.NopCloser(bytes.NewReader(body))
	}
	return d.base.RoundTrip(req)
}

func main() {
	fmt.Println("=== Generic Raw API 示例：两种接入模式 ===")
	fmt.Println("提示：默认不发真实网络请求；设置 RUN_LIVE_CHAT=1 可执行真实对话。")
	fmt.Println()

	c, _ := setup()
	ctx := context.Background()
	runLive := true

	//demoHTTPSpecModes(ctx, c, configs, runLive)
	//demoMultiRoundInference(c)
	demoMultiRoundFromFile(ctx, c, runLive)
}

// setup 读取配置文件并初始化 HTTP 客户端。
func setup() (*client.Client, map[string][]generic.RawHTTPSpec) {
	_, srcFile, _, _ := runtime.Caller(0)
	jsonPath := filepath.Join(filepath.Dir(srcFile), "request_json.json")

	data, err := os.ReadFile(jsonPath)
	if err != nil {
		log.Fatalf("读取配置文件失败: %v", err)
	}

	var configs map[string][]generic.RawHTTPSpec
	if err := json.Unmarshal(data, &configs); err != nil {
		log.Fatalf("解析配置文件失败: %v", err)
	}

	c := client.New()
	innerTransport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
	}
	c.HTTP = &http.Client{
		Timeout:   120 * time.Second,
		Transport: &debugTransport{base: innerTransport},
	}

	return c, configs
}

func demoHTTPSpecModes(ctx context.Context, c *client.Client, configs map[string][]generic.RawHTTPSpec, runLive bool) {
	fmt.Println("=== 一、NewSessionFromHTTPSpec（remote_session + local_history）===")

	if specs := configs["remote_session"]; len(specs) > 0 {
		fmt.Println("\n[HTTPSpec] remote_session")
		demoRemoteSession(ctx, c, specs[0], runLive)
	} else {
		fmt.Println("[HTTPSpec] 未找到 remote_session 配置")
	}

	//if specs := configs["local_history"]; len(specs) > 0 {
	//	fmt.Println("\n[HTTPSpec] local_history")
	//	demoLocalHistory(ctx, c, specs[0], runLive)
	//} else {
	//	fmt.Println("[HTTPSpec] 未找到 local_history 配置")
	//}
}

// demoRemoteSession 演示 remote_session 模式（服务端维护会话，多轮对话）。
func demoRemoteSession(ctx context.Context, c *client.Client, spec generic.RawHTTPSpec, runLive bool, opts ...client.SessionOption) bool {
	sess, err := c.NewSessionFromHTTPSpec(spec, opts...)
	if err != nil {
		log.Printf("创建 remote_session Session 失败: %v", err)
		return false
	}

	fmt.Printf("创建成功，当前 session ID: %q\n", sess.ID())
	if !runLive {
		fmt.Println("已跳过真实请求（设置 RUN_LIVE_CHAT=1 可运行 chatStream）。")
		return true
	}

	fmt.Println("\n── 第一轮 ──")
	fmt.Printf("发送前 session ID: %q（空 = 首轮，服务端创建会话）\n", sess.ID())
	if _, err := chatStream(ctx, sess, "你好，请用20个字简单介绍一下你自己"); err != nil {
		log.Printf("第一轮请求失败: %v", err)
		return false
	}

	fmt.Println("\n\n── 第二轮 ──")
	fmt.Printf("发送前 session ID: %q（注入 session_id + chain 字段）\n", sess.ID())
	if _, err := chatStream(ctx, sess, "你刚才说了什么？"); err != nil {
		log.Printf("第二轮请求失败: %v", err)
		return false
	}
	fmt.Println()
	return true
}

// demoLocalHistory 演示 local_history 模式（本地维护历史，多轮对话）。
// SDK 在每轮请求前自动从文件 store 加载历史消息并拼入 messages，无需业务层手动管理。
func demoLocalHistory(ctx context.Context, c *client.Client, spec generic.RawHTTPSpec, runLive bool) bool {
	_, srcFile, _, _ := runtime.Caller(0)
	store := sessionstore.NewFile(sessionstore.FileConfig{
		Path: filepath.Join(filepath.Dir(srcFile), "..", "sessions.json"),
	})

	sess, err := c.NewSessionFromHTTPSpec(spec,
		client.WithStore(store),
		client.WithAutoID(),
	)
	if err != nil {
		log.Printf("创建 local_history Session 失败: %v", err)
		return false
	}
	fmt.Printf("创建成功，本地 session ID: %q\n", sess.ID())

	if !runLive {
		fmt.Println("已跳过真实请求（设置 RUN_LIVE_CHAT=1 可运行 chatStream）。")
		return true
	}

	fmt.Println("\n── 第一轮 ──")
	if _, err := chatStream(ctx, sess, "请用20个字介绍一下Go语言"); err != nil {
		log.Printf("第一轮请求失败: %v", err)
		return false
	}
	fmt.Println("\n\n── 第二轮 ──")
	if _, err := chatStream(ctx, sess, "你刚才说的特点中，最重要的是哪一个？请用20个字回答。"); err != nil {
		log.Printf("第二轮请求失败: %v", err)
		return false
	}
	fmt.Println()
	return true
}

// demoMultiRoundInference 演示 NewSessionFromMultiRound 自动推理。
// 覆盖 auto_confirmed / pending_confirm / failed(以 error 返回) 三种路径。
func demoMultiRoundInference(c *client.Client) {
	fmt.Println("\n=== 二、NewSessionFromMultiRound（自动推理，三路径）===")

	cases := []struct {
		name     string
		expected string
		spec     generic.MultiRoundSpec
	}{
		{name: "auto_confirmed", expected: "auto_confirmed", spec: buildMultiRoundSpecAuto()},
		{name: "pending_confirm", expected: "pending_confirm", spec: buildMultiRoundSpecPending()},
		{name: "failed", expected: "error", spec: buildMultiRoundSpecError()},
	}

	for _, tc := range cases {
		fmt.Printf("\n[MultiRound] 用例：%s（期望：%s）\n", tc.name, tc.expected)
		sess, inferred, err := c.NewSessionFromMultiRound(tc.spec)
		actual := handleInferenceResult(sess, inferred, err)
		if actual == tc.expected {
			fmt.Printf("路径校验：PASS（%s）\n", actual)
		} else {
			fmt.Printf("路径校验：FAIL（期望=%s，实际=%s）\n", tc.expected, actual)
		}
	}
}

// demoMultiRoundFromFile 演示从 multi_round_spec.json 加载抓包原文多轮配置并做推理报告。
// 该流程不会发起真实网络请求，仅验证配置可读和推理路径。
func demoMultiRoundFromFile(ctx context.Context, c *client.Client, runLive bool) {
	fmt.Println("\n=== 三、从 JSON 文件加载抓包原文多轮配置（仅推理，不发请求）===")

	_, srcFile, _, _ := runtime.Caller(0)
	specPath := filepath.Join(filepath.Dir(srcFile), "multi_round_spec.json")
	exportPath := filepath.Join(filepath.Dir(srcFile), "new_request_json.json")
	spec, err := loadHTTPMultiRoundSpecFromFile(specPath)
	if err != nil {
		fmt.Printf("读取 multi_round_spec.json 失败：%v\n", err)
		return
	}

	fmt.Printf("[HTTPMultiRound-File] 已加载：%s\n", specPath)
	sess, inferred, inferErr := c.NewSessionFromHTTPMultiRound(spec)
	result := handleInferenceResult(sess, inferred, inferErr)
	fmt.Printf("[HTTPMultiRound-File] 路径结果：%s\n", result)
	if result == "error" {
		return
	}

	if err := exportInferredRequestJSON(spec, inferred, exportPath); err != nil {
		fmt.Printf("[HTTPMultiRound-File] 导出 new_request_json.json 失败：%v\n", err)
		return
	}
	fmt.Printf("[HTTPMultiRound-File] 导出成功：%s\n", exportPath)

	// 初始化 sessions.json 文件存储
	_, srcFile2, _, _ := runtime.Caller(0)
	store := sessionstore.NewFile(sessionstore.FileConfig{
		Path: filepath.Join(filepath.Dir(srcFile2), "..", "sessions.json"),
	})

	replayMode := inferConversationMode(strings.TrimSpace(spec.Model), inferred)
	fmt.Printf("[HTTPMultiRound-File] 回放模式：%s\n", replayMode)
	fmt.Println("[HTTPMultiRound-File] 开始回放验证（From HTTPSpec）")
	exportedSpec, err := loadFirstSpecByModeFromFile(exportPath, replayMode)
	if err != nil {
		fmt.Printf("[HTTPMultiRound-File] 验证结果：FAIL（%v）\n", err)
		return
	}

	var ok bool
	switch replayMode {
	case "local_history":
		ok = demoLocalHistory(ctx, c, exportedSpec, runLive)
	default:
		ok = demoRemoteSession(ctx, c, exportedSpec, runLive, client.WithStore(store))
	}
	if ok {
		fmt.Println("[HTTPMultiRound-File] 验证结果：PASS")
		return
	}
	fmt.Println("[HTTPMultiRound-File] 验证结果：FAIL")
}

func loadHTTPMultiRoundSpecFromFile(path string) (generic.RawHTTPMultiRoundSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return generic.RawHTTPMultiRoundSpec{}, fmt.Errorf("读取文件失败: %w", err)
	}
	var spec generic.RawHTTPMultiRoundSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return generic.RawHTTPMultiRoundSpec{}, fmt.Errorf("解析 JSON 失败: %w", err)
	}
	return spec, nil
}

func loadFirstSpecByModeFromFile(path, mode string) (generic.RawHTTPSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return generic.RawHTTPSpec{}, fmt.Errorf("读取导出文件失败: %w", err)
	}

	var payload map[string][]generic.RawHTTPSpec
	if err := json.Unmarshal(data, &payload); err != nil {
		return generic.RawHTTPSpec{}, fmt.Errorf("解析导出 JSON 失败: %w", err)
	}

	mode = normalizeConversationMode(mode)
	specs := payload[mode]
	if len(specs) == 0 {
		return generic.RawHTTPSpec{}, fmt.Errorf("导出文件缺少 %s 首条 spec", mode)
	}
	return specs[0], nil
}

type rawHeader struct {
	Key   string
	Value string
}

func exportInferredRequestJSON(rawSpec generic.RawHTTPMultiRoundSpec, inferred *generic.InferredIntegration, outputPath string) error {
	payload, err := buildRequestJSONPayload(rawSpec, inferred)
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化导出 JSON 失败: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("写入导出文件失败: %w", err)
	}
	return nil
}

func buildRequestJSONPayload(rawSpec generic.RawHTTPMultiRoundSpec, inferred *generic.InferredIntegration) (map[string][]generic.RawHTTPSpec, error) {
	if inferred == nil || inferred.Profile == nil {
		return nil, fmt.Errorf("推理结果为空，无法导出")
	}
	if len(rawSpec.Rounds) == 0 {
		return nil, fmt.Errorf("rounds 为空，无法导出")
	}

	method, headers, err := parseRawRequestMeta(rawSpec.Rounds[0].Request)
	if err != nil {
		return nil, err
	}

	requestPath := strings.TrimSpace(inferred.Profile.Request.Path)
	if requestPath == "" {
		requestPath = resolvePathFromURL(rawSpec.BaseURL)
	}
	if requestPath == "" {
		requestPath = "/"
	}

	chainFields := append([]generic.ChainField(nil), inferred.Profile.Response.Stream.ChainFields...)
	bodyTemplate, chainFields := normalizeExportBodyTemplate(inferred.Profile.Request.BodyTemplate, rawSpec.Rounds, chainFields)

	bodyText, err := renderRequestBodyForRawSpec(bodyTemplate)
	if err != nil {
		return nil, err
	}

	requestText := buildRawRequestText(method, requestPath, headers, bodyText)
	mode := inferConversationMode(strings.TrimSpace(rawSpec.Model), inferred)

	exportSpec := generic.RawHTTPSpec{
		Model:       mode,
		BaseURL:     resolveExportBaseURL(rawSpec.BaseURL, inferred.BaseURL, requestPath),
		Request:     requestText,
		Response:    rawSpec.Rounds[0].Response,
		ChainFields: chainFields,
	}

	payload := map[string][]generic.RawHTTPSpec{
		mode: {exportSpec},
	}
	return payload, nil
}

func inferConversationMode(rawModel string, inferred *generic.InferredIntegration) string {
	if inferred != nil && inferred.Profile != nil {
		mode := strings.TrimSpace(inferred.Profile.Conversation.Mode)
		if mode != "" {
			return normalizeConversationMode(mode)
		}
	}
	return normalizeConversationMode(rawModel)
}

func normalizeConversationMode(mode string) string {
	mode = strings.TrimSpace(mode)
	if mode == "" {
		return "remote_session"
	}
	return mode
}

func normalizeExportBodyTemplate(bodyTemplate map[string]any, rounds []generic.RawHTTPRound, chainFields []generic.ChainField) (map[string]any, []generic.ChainField) {
	body := cloneAnyMap(bodyTemplate)
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
		chainFields = append(chainFields, generic.ChainField{
			Placeholder:  ph,
			ResponsePath: "req_id",
		})
	}

	if hasReqIDInAnyRound(rounds) {
		body["req_id"] = "{{uuid}}"
	}

	return body, chainFields
}

func hasReqIDInAnyRound(rounds []generic.RawHTTPRound) bool {
	for _, round := range rounds {
		if hasTopLevelJSONField(round.Request, "req_id") {
			return true
		}
	}
	return false
}

func hasTopLevelJSONField(rawRequest, field string) bool {
	body, err := extractRawRequestBody(rawRequest)
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

func extractRawRequestBody(rawRequest string) (string, error) {
	text := strings.ReplaceAll(rawRequest, "\r\n", "\n")
	parts := strings.SplitN(text, "\n\n", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("raw request 缺少 body 段")
	}
	return strings.TrimSpace(parts[1]), nil
}

func cloneAnyMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = cloneAnyValue(v)
	}
	return out
}

func cloneAnyValue(v any) any {
	switch val := v.(type) {
	case map[string]any:
		return cloneAnyMap(val)
	case []any:
		out := make([]any, len(val))
		for i, item := range val {
			out[i] = cloneAnyValue(item)
		}
		return out
	default:
		return v
	}
}

func parseRawRequestMeta(rawRequest string) (string, []rawHeader, error) {
	text := strings.ReplaceAll(rawRequest, "\r\n", "\n")
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		return "", nil, fmt.Errorf("首轮 request 为空")
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
		return "", nil, fmt.Errorf("首轮 request 缺少请求行")
	}

	parts := strings.Fields(first)
	if len(parts) == 0 {
		return "", nil, fmt.Errorf("首轮 request 请求行非法")
	}
	method := strings.ToUpper(strings.TrimSpace(parts[0]))
	if method == "" {
		method = "POST"
	}

	var headers []rawHeader
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
		headers = append(headers, rawHeader{Key: key, Value: value})
	}

	sort.SliceStable(headers, func(i, j int) bool {
		return strings.ToLower(headers[i].Key) < strings.ToLower(headers[j].Key)
	})
	return method, headers, nil
}

func renderRequestBodyForRawSpec(bodyTemplate map[string]any) (string, error) {
	rawBody := normalizeIndexedObjectToArray(toRawPlaceholders(bodyTemplate))
	data, err := json.MarshalIndent(rawBody, "", "  ")
	if err != nil {
		return "", fmt.Errorf("序列化导出请求体失败: %w", err)
	}
	return string(data), nil
}

func toRawPlaceholders(v any) any {
	switch val := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(val))
		for k, item := range val {
			out[k] = toRawPlaceholders(item)
		}
		return out
	case []any:
		out := make([]any, len(val))
		for i, item := range val {
			out[i] = toRawPlaceholders(item)
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

func normalizeIndexedObjectToArray(v any) any {
	switch val := v.(type) {
	case map[string]any:
		normalized := make(map[string]any, len(val))
		for k, item := range val {
			normalized[k] = normalizeIndexedObjectToArray(item)
		}
		if arr, ok := mapWithContiguousIndexToArray(normalized); ok {
			return arr
		}
		return normalized
	case []any:
		out := make([]any, len(val))
		for i, item := range val {
			out[i] = normalizeIndexedObjectToArray(item)
		}
		return out
	default:
		return v
	}
}

func mapWithContiguousIndexToArray(m map[string]any) ([]any, bool) {
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

func buildRawRequestText(method, path string, headers []rawHeader, body string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s HTTP/1.1\n", method, path)
	for _, h := range headers {
		fmt.Fprintf(&b, "%s: %s\n", h.Key, h.Value)
	}
	b.WriteString("\n")
	b.WriteString(body)
	return b.String()
}

func resolveExportBaseURL(rawBaseURL, inferredBaseURL, requestPath string) string {
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

func resolvePathFromURL(raw string) string {
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

// handleInferenceResult 统一处理自动推理三条路径：
// 1) auto_confirmed: 返回可用 session
// 2) pending_confirm: 返回可用 session，但需人工复核报告
// 3) error: 返回 err（如报文非法、推理失败）
func handleInferenceResult(sess *client.Session, inferred *generic.InferredIntegration, err error) string {
	if err != nil {
		fmt.Printf("结果：error，错误信息：%v\n", err)
		if inferred != nil && inferred.Report != nil {
			printInferenceReport(inferred.Report)
		}
		return "error"
	}

	if inferred == nil || inferred.Report == nil {
		fmt.Println("结果：error，未返回推理报告")
		return "error"
	}

	switch inferred.Report.Status {
	case "auto_confirmed":
		if sess == nil {
			fmt.Println("结果：error，状态为 auto_confirmed 但未返回 Session")
			printInferenceReport(inferred.Report)
			return "error"
		}
		fmt.Println("结果：auto_confirmed，已自动创建 Session")
		fmt.Printf("Session ID（初始）: %q\n", sess.ID())
		printInferenceReport(inferred.Report)
		return "auto_confirmed"
	case "pending_confirm":
		if sess == nil {
			fmt.Println("结果：error，状态为 pending_confirm 但未返回 Session")
			printInferenceReport(inferred.Report)
			return "error"
		}
		fmt.Println("结果：pending_confirm，已创建 Session，但需人工复核报告")
		fmt.Printf("Session ID（初始）: %q\n", sess.ID())
		printInferenceReport(inferred.Report)
		return "pending_confirm"
	case "failed":
		fmt.Println("结果：error（推理状态为 failed），建议回退 RawIntegrationSpec 手工接入")
		printInferenceReport(inferred.Report)
		return "error"
	default:
		fmt.Printf("结果：error，未知状态：%s\n", inferred.Report.Status)
		printInferenceReport(inferred.Report)
		return "error"
	}
}

// printInferenceReport 打印推理报告核心信息。
func printInferenceReport(report *generic.InferenceReport) {
	fmt.Println("推理报告：")
	fmt.Printf("  status: %s\n", report.Status)
	fmt.Printf("  confidence: %.4f\n", report.OverallConfidence)
	fmt.Printf("  fields(%d):\n", len(report.Fields))
	for i, f := range report.Fields {
		fmt.Printf("    %d) class=%s request=%s response=%s placeholder=%s confidence=%.2f\n",
			i+1, f.Class, f.RequestPath, f.ResponsePath, f.Placeholder, f.Confidence)
	}
	if len(report.Warnings) > 0 {
		fmt.Printf("  warnings(%d):\n", len(report.Warnings))
		for i, w := range report.Warnings {
			fmt.Printf("    %d) %s\n", i+1, w)
		}
	}
	if len(report.Suggestions) > 0 {
		fmt.Printf("  suggestions(%d):\n", len(report.Suggestions))
		for i, s := range report.Suggestions {
			fmt.Printf("    %d) target=%s action=%s value=%s reason=%s priority=%s\n",
				i+1, s.Target, s.Action, s.Value, s.Reason, s.Priority)
		}
	}
}

// buildMultiRoundSpecAuto: 明确占位符样本，预期 auto_confirmed。
// 覆盖 input/session_id/chain/static 四种字段形态。
func buildMultiRoundSpecAuto() generic.MultiRoundSpec {
	return generic.MultiRoundSpec{
		URL: "https://mock.example.com/v1/chat/completions",
		Conversation: generic.ConversationProfile{
			Mode: "remote_session",
		},
		Rounds: []generic.RoundPair{
			{
				Request:  generic.RawPacket{Headers: map[string]string{"Authorization": "Bearer <MOCK_TOKEN>"}, Body: `{"prompt":"$$$","session_id":"","parent_message_id":"","model":"demo-model"}`},
				Response: generic.RawPacket{Body: `{"content":"你好","session_id":"sess-001","message_id":"msg-001"}`},
			},
			{
				Request:  generic.RawPacket{Body: `{"prompt":"$$$","session_id":"sess-001","parent_message_id":"msg-001","model":"demo-model"}`},
				Response: generic.RawPacket{Body: `{"content":"继续回答","session_id":"sess-001","message_id":"msg-002"}`},
			},
			{
				Request:  generic.RawPacket{Body: `{"prompt":"$$$","session_id":"sess-001","parent_message_id":"msg-002","model":"demo-model"}`},
				Response: generic.RawPacket{Body: `{"content":"第三轮","session_id":"sess-001","message_id":"msg-003"}`},
			},
		},
	}
}

// buildMultiRoundSpecPending: 输入字段未显式使用 $$$，降低置信度，预期 pending_confirm。
func buildMultiRoundSpecPending() generic.MultiRoundSpec {
	return generic.MultiRoundSpec{
		URL: "https://mock.example.com/v1/chat/completions",
		Conversation: generic.ConversationProfile{
			Mode: "remote_session",
		},
		Rounds: []generic.RoundPair{
			{
				Request:  generic.RawPacket{Body: `{"prompt":"你好","session_id":"","parent_message_id":"","model":"demo-model"}`},
				Response: generic.RawPacket{Body: `{"content":"你好","session_id":"sess-101","message_id":"msg-101"}`},
			},
			{
				Request:  generic.RawPacket{Body: `{"prompt":"请继续","session_id":"sess-101","parent_message_id":"msg-101","model":"demo-model"}`},
				Response: generic.RawPacket{Body: `{"content":"继续","session_id":"sess-101","message_id":"msg-102"}`},
			},
			{
				Request:  generic.RawPacket{Body: `{"prompt":"总结一下","session_id":"sess-101","parent_message_id":"msg-102","model":"demo-model"}`},
				Response: generic.RawPacket{Body: `{"content":"总结","session_id":"sess-101","message_id":"msg-103"}`},
			},
		},
	}
}

// buildMultiRoundSpecError: 构造可解析但无法分类字段的样本，触发 failed（以 error 返回）。
func buildMultiRoundSpecError() generic.MultiRoundSpec {
	return generic.MultiRoundSpec{
		URL: "https://mock.example.com/v1/chat/completions",
		Conversation: generic.ConversationProfile{
			Mode: "remote_session",
		},
		Rounds: []generic.RoundPair{
			{
				Request:  generic.RawPacket{Body: `{}`},
				Response: generic.RawPacket{Body: `{}`},
			},
			{
				Request:  generic.RawPacket{Body: `{}`},
				Response: generic.RawPacket{Body: `{}`},
			},
		},
	}
}

// chatStream 发送流式请求并打印 delta，返回完整回答。
func chatStream(ctx context.Context, sess *client.Session, question string) (string, error) {
	fmt.Printf("问：%s\n答：", question)

	stream, err := sess.ChatStream(ctx, base.ChatRequest{
		Messages: []base.Message{
			{Role: "user", Content: question},
		},
	})
	if err != nil {
		return "", fmt.Errorf("发起流式请求失败: %w", err)
	}

	var full string
	for chunk := range stream {
		if chunk.Error != nil {
			return full, fmt.Errorf("流式接收错误: %w", chunk.Error)
		}
		if chunk.Text != "" {
			fmt.Print(chunk.Text)
			full += chunk.Text
		}
	}
	return full, nil
}
