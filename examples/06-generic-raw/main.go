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
	"os"
	"path/filepath"
	"runtime"
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
	fmt.Println("=== Generic Raw API 示例 ===")
	fmt.Println("提示：默认不发真实网络请求；设置 RUN_LIVE_CHAT=1 可执行真实对话。")
	fmt.Println()

	c, configs := setup()
	ctx := context.Background()
	runLive := true

	// 一、从已有 HTTPSpec 配置创建 Session（手动构造场景）
	demoHTTPSpecModes(ctx, c, configs, runLive)

	// 二、MultiRound 自动推理三条路径
	demoMultiRoundInference(c)

	// 三、RawReasoning → Quick：从抓包到可用会话的一步到位流程
	demoRawReasoningToQuick(ctx, c, runLive)
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

// ─── 一、HTTPSpec 手动模式 ───

func demoHTTPSpecModes(ctx context.Context, c *client.Client, configs map[string][]generic.RawHTTPSpec, runLive bool) {
	fmt.Println("=== 一、NewSessionFromHTTPSpec（remote_session + local_history）===")

	if specs := configs["remote_session"]; len(specs) > 0 {
		fmt.Println("\n[HTTPSpec] remote_session")
		demoRemoteSession(ctx, c, specs[0], runLive)
	} else {
		fmt.Println("[HTTPSpec] 未找到 remote_session 配置")
	}

	if specs := configs["local_history"]; len(specs) > 0 {
		fmt.Println("\n[HTTPSpec] local_history")
		demoLocalHistory(ctx, c, specs[0], runLive)
	} else {
		fmt.Println("[HTTPSpec] 未找到 local_history 配置")
	}
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

// ─── 二、MultiRound 自动推理（三路径）───

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

// ─── 三、RawReasoning → Quick（推荐流程）───

// demoRawReasoningToQuick 演示从抓包原文到可用会话的完整闭环：
//
//	RawReasoning(多轮抓包) → RawHTTPSpec → Quick(ProviderConfig) → QuickSession
//
// 这是 Generic 接入的推荐方式，无需手动构造 profile 或分步调用推理/导出 API。
func demoRawReasoningToQuick(ctx context.Context, c *client.Client, runLive bool) {
	fmt.Println("\n=== 三、RawReasoning → Quick（从抓包到会话，一步到位）===")

	// 1. 加载多轮抓包原文
	_, srcFile, _, _ := runtime.Caller(0)
	specPath := filepath.Join(filepath.Dir(srcFile), "multi_round_spec.json")
	rawSpec, err := loadHTTPMultiRoundSpecFromFile(specPath)
	if err != nil {
		fmt.Printf("读取 multi_round_spec.json 失败：%v\n", err)
		return
	}
	fmt.Printf("[RawReasoning] 已加载抓包文件：%s\n", specPath)

	// 2. 一步完成推理 + 导出
	spec, err := c.RawReasoning(rawSpec)
	if err != nil {
		fmt.Printf("[RawReasoning] 推理失败：%v\n", err)
		return
	}
	fmt.Printf("[RawReasoning] 推理成功：mode=%s, base_url=%s\n", spec.Model, spec.BaseURL)

	// 可选：持久化导出结果供后续复用
	exportPath := filepath.Join(filepath.Dir(srcFile), "new_request_json.json")
	if err := saveExportedSpec(spec, exportPath); err != nil {
		fmt.Printf("[RawReasoning] 保存导出文件失败：%v\n", err)
	} else {
		fmt.Printf("[RawReasoning] 已导出：%s\n", exportPath)
	}

	// 3. 直接传入 Quick 创建会话
	qs, err := c.Quick(client.ProviderConfig{
		Provider:    "generic",
		BaseURL:     spec.BaseURL,
		SessionMode: spec.Model,
		Request:     spec.Request,
		Response:    spec.Response,
		ChainFields: spec.ChainFields,
	})
	if err != nil {
		fmt.Printf("[RawReasoning] Quick 创建会话失败：%v\n", err)
		return
	}
	fmt.Printf("[RawReasoning] Quick 会话创建成功，模式：%s\n", spec.Model)

	if !runLive {
		fmt.Println("[RawReasoning] 已跳过真实请求（设置 RUN_LIVE_CHAT=1 可运行对话）。")
		return
	}

	// 4. 使用 QuickSession 进行多轮对话
	fmt.Println("\n── 第一轮 ──")
	if _, err := quickChat(ctx, qs, "你好，请用20个字简单介绍一下你自己"); err != nil {
		fmt.Printf("[RawReasoning] 第一轮请求失败：%v\n", err)
		return
	}
	fmt.Println("\n\n── 第二轮 ──")
	if _, err := quickChat(ctx, qs, "你刚才说了什么？"); err != nil {
		fmt.Printf("[RawReasoning] 第二轮请求失败：%v\n", err)
		return
	}
	fmt.Println("\n[RawReasoning] 验证结果：PASS")
}

// ─── 工具函数 ───

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

func saveExportedSpec(spec *generic.RawHTTPSpec, path string) error {
	payload := map[string][]generic.RawHTTPSpec{
		spec.Model: {*spec},
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化导出 JSON 失败: %w", err)
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
}

// handleInferenceResult 统一处理自动推理三条路径。
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

// chatStream 发送流式请求并打印 delta（Session 级别）。
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

// quickChat 通过 QuickSession 发送流式请求并打印 delta。
func quickChat(ctx context.Context, qs *client.QuickSession, question string) (string, error) {
	fmt.Printf("问：%s\n答：", question)

	stream, err := qs.SendText(ctx, question)
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

// ─── MultiRound 推理测试用例 ───

// buildMultiRoundSpecAuto: 明确占位符样本，预期 auto_confirmed。
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

// buildMultiRoundSpecError: 构造可解析但无法分类字段的样本，触发 failed。
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
