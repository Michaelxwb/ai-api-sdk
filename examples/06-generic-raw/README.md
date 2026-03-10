# 06-generic-raw 示例说明

## 概述
本示例展示如何用 `generic raw` 方式接入非标准模型 API，覆盖两种接入模式：
- `NewSessionFromHTTPSpec`：基于抓包原始 HTTP 文本
- `NewSessionFromHTTPMultiRound`：基于 2~5 轮抓包原文自动推理（SDK 内部自动转换到 `MultiRoundSpec`）

示例代码在 [`main.go`](/Users/jahan/workspace/ai-api-sdk/examples/06-generic-raw/main.go)。
配置文件包括：
- [`request_json.json`](/Users/jahan/workspace/ai-api-sdk/examples/06-generic-raw/request_json.json)（HTTPSpec 示例）
- [`multi_round_spec.json`](/Users/jahan/workspace/ai-api-sdk/examples/06-generic-raw/multi_round_spec.json)（抓包原文多轮配置示例）

## 两种接入模式

### 1) HTTPSpec（remote_session + local_history）
- 输入：`model/base_url/request/response/chain_fields`
- 优点：兼容已有抓包数据，接入快
- 适合：已明确请求模板和响应解析路径

### 2) MultiRound（自动推理）
- 输入：`RawHTTPMultiRoundSpec`（2~5 轮 request/response 原文）
- SDK 内部自动转换：`RawHTTPMultiRoundSpec -> MultiRoundSpec`
- 自动识别字段分类：`input/session_id/chain/static/dynamic`
- 输出：`InferredIntegration + InferenceReport`

## 为什么要单独 JSON 文件
将 `MultiRoundSpec` 放在独立的 `multi_round_spec.json`，可以把“业务报文样本”与“代码逻辑”解耦：
- 业务方可直接维护配置，不必改 Go 代码
- 联调时可快速替换样本，便于版本管理与评审
- 同一套加载逻辑可复用于测试、灰度和生产接入流程

## 抓包原文多轮 最小可运行步骤
1. 准备 `multi_round_spec.json`
2. 反序列化为 `generic.RawHTTPMultiRoundSpec`
3. 调用 `c.NewSessionFromHTTPMultiRound(spec)`（SDK 内部自动转换）
4. 处理 `auto_confirmed / pending_confirm / failed(以 error 返回)`

完整可复制代码（中文注释）：

```go
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/Michaelxwb/ai-api-sdk/client"
	"github.com/Michaelxwb/ai-api-sdk/provider/impls/generic"
)

func demoMultiRoundFromJSONFile(c *client.Client, jsonPath string) {
	// 1) 读取 JSON 文件
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		fmt.Printf("读取失败: %v\n", err)
		return
	}

	// 2) 反序列化为抓包原文多轮结构
	var spec generic.RawHTTPMultiRoundSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		fmt.Printf("反序列化失败: %v\n", err)
		return
	}

	// 3) 调用 NewSessionFromHTTPMultiRound（只做推理，不会自动发真实请求）
	// SDK 内部会自动把 request/response 原文转换成 MultiRoundSpec。
	sess, inferred, err := c.NewSessionFromHTTPMultiRound(spec)
	if err != nil {
		// 4-a) error: 报文非法，或推理状态 failed（会以 error 返回）
		fmt.Printf("结果: error, err=%v\n", err)
		if inferred != nil && inferred.Report != nil {
			fmt.Printf("推理状态: %s\n", inferred.Report.Status)
		}
		return
	}
	if inferred == nil || inferred.Report == nil {
		fmt.Println("结果: error, 推理报告为空")
		return
	}

	switch inferred.Report.Status {
	case "auto_confirmed":
		// 4-b) auto_confirmed: 已自动创建 Session
		fmt.Printf("结果: auto_confirmed, session_created=%v\n", sess != nil)
	case "pending_confirm":
		// 4-c) pending_confirm: 仍会创建 Session，但需人工复核报告
		fmt.Printf("结果: pending_confirm, session_created=%v, 请人工复核字段映射\n", sess != nil)
	default:
		fmt.Printf("结果: error, 未知状态=%s\n", inferred.Report.Status)
	}
}
```

## 两轮样本说明
两轮样本也直接使用抓包原文多轮配置（`rounds` 传 2 条即可），SDK 内部仍走 `MultiRoundSpec` 推理路线，不存在独立 `TwoRound` 业务配置方案。

## 运行方式

```bash
# 仅构建
go build ./examples/06-generic-raw/

# 运行示例（默认跳过真实网络请求）
go run ./examples/06-generic-raw/

# 执行真实流式请求（需先填写 request_json.json 中凭证）
RUN_LIVE_CHAT=1 go run ./examples/06-generic-raw/
```

## API 参考
- `(*client.Client).NewSessionFromHTTPSpec(spec generic.RawHTTPSpec, opts ...client.SessionOption)`
- `(*client.Client).NewSessionFromHTTPMultiRound(spec generic.RawHTTPMultiRoundSpec, opts ...client.SessionOption)`
- `(*client.Client).NewSessionFromMultiRound(spec generic.MultiRoundSpec, opts ...client.SessionOption)`
- `generic.InferenceReport`
  - `Status`
  - `OverallConfidence`
  - `Fields`
  - `Warnings`
  - `Suggestions`
