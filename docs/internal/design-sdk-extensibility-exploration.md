# SDK 扩展能力可行性探索

> 评估当前 SDK 对两个未来场景的适配能力，暂不实施，仅做方案探索。

---

## 一、多模态适配（图片/附件/音频）

### 现状
- `base/types.go:4-9` — `Message.Content` 为 `string`，被 23 处直接作为字符串使用
- 9 个 Provider 的 `BuildRequest` 均假设 Content 是纯文本
- Plugin Transport (`transport.go:459-482`) 已有 `coerceToString()` 支持 `interface{}`，有一定基础

### 推荐方案：并存方案

```go
type Message struct {
    Role       string        `json:"role"`
    Content    string        `json:"content"`           // 保留，向下兼容
    Parts      []ContentPart `json:"parts,omitempty"`   // 新增：多模态
    Name       string        `json:"name,omitempty"`
    ToolCallID string        `json:"tool_call_id,omitempty"`
}

type ContentPart struct {
    Type     string         `json:"type"`               // "text", "image", "document"
    Text     string         `json:"text,omitempty"`
    URL      string         `json:"url,omitempty"`
    Data     string         `json:"data,omitempty"`      // base64
    MIMEType string         `json:"mime_type,omitempty"`
    Metadata map[string]any `json:"metadata,omitempty"`
}
```

- `Content != ""` → 旧行为，纯文本
- `len(Parts) > 0` → 新行为，多模态
- 应用层统一通过 `Parts` 传入文本/图片/附件/音频
- 各 Provider 的 `BuildRequest` 各自决定内联还是先上传拿 file_id

### 改造范围

| 层 | 影响 | 说明 |
|---|---|---|
| `base/types.go` | 小 | 新增 ContentPart + Parts 字段 |
| Provider BuildRequest | 中-大 | 9 个 Provider 逐个适配 |
| Generic Adapter 模板层 | 中 | 模板需支持 Parts 展开 |
| Client/Session | 几乎无 | 只透传 Message |

### 各 Provider 格式映射

| Provider | 官方多模态支持 | 格式特点 |
|---|---|---|
| OpenAI | 图片/文件/音频 | `content: [{type: "text"}, {type: "image_url"}]` |
| Claude | 图片/PDF/文件 | `content: [{type: "text"}, {type: "image", source: {type: "base64"}}]` |
| Gemini | 图片/文件 | `parts: [{text}, {inlineData: {mimeType, data}}]` |
| Dify | 图片 | 需确认具体格式 |
| FastGPT/RAGFlow | 需确认 | 需文档调研 |

### 关键决策点（实施时确认）
1. SDK 层是否做统一文件大小校验，还是交 Provider 各自处理
2. 流式响应中的多模态（如 DALL-E 返回图片）是否需要支持

### 结论
**可行性：高**。并存方案零破坏，Provider 层逐个适配，互不影响。

---

## 二、单轮内多步编排（Multi-Step Pipeline）

### 场景
用户发一次消息，SDK 内部需要串联调用多个接口才能得到最终答案。例如：
1. 调接口 A（发送问题）→ 拿到 `task_id`
2. 调接口 B（传入 `task_id`）→ 拿到最终答案

应用层只感知一次 `Chat()` 调用，多步编排由 SDK 内部完成。

### 现状
- **ChainField 机制**（`generic/profile.go:6-8`）：已支持**跨轮次**的字段传递（第 N 轮提取 → 第 N+1 轮注入），但不支持单轮内多步
- **Rounds**（`generic/rawpacket.go:34-41`）：用于推理/配置生成阶段的多轮抓包解析，不是运行时编排
- **GenericProfile** 只包含一组请求/响应配置，无 pipeline/steps 概念

### 需要新增的能力

#### 方案：GenericProfile 支持 Steps

```go
type GenericProfile struct {
    // 现有字段...

    // Steps 定义单轮内多步编排，为空时保持当前单步行为。
    // 每个 Step 是一组独立的请求/响应配置，步骤间通过 ChainField 传递数据。
    Steps []StepProfile `json:"steps,omitempty" yaml:"steps,omitempty"`
}

type StepProfile struct {
    Name     string            `json:"name"`      // 步骤名，如 "send_question", "get_answer"
    URL      string            `json:"url"`       // 可选，不同步骤可能调不同接口
    Method   string            `json:"method"`
    Headers  map[string]string `json:"headers"`
    Body     map[string]any    `json:"body"`      // 模板，支持 $$$PLACEHOLDER$$$
    Response StepResponse      `json:"response"`
}

type StepResponse struct {
    // 从此步响应中提取值，注入下一步请求模板
    ExtractFields []ChainField `json:"extract_fields"`
    // 是否为最终步骤（从此步提取最终文本）
    IsFinal       bool         `json:"is_final"`
    // 最终文本提取路径（仅 IsFinal=true 时有效）
    TextPath      string       `json:"text_path"`
    Stream        StreamProfile `json:"stream"`
}
```

#### 运行时执行流程

```
GenericSpec.BuildRequest() / ParseResponse() 改造为：

if len(profile.Steps) == 0:
    // 现有单步逻辑，不变
else:
    stepValues := map[string]string{}
    for i, step := range profile.Steps:
        req := renderTemplate(step.Body, input, stepValues)
        resp := httpClient.Do(req)
        stepValues = extractFields(resp, step.Response.ExtractFields)
        if step.Response.IsFinal:
            return parseText(resp, step.Response.TextPath)
```

### 改造范围

| 层 | 影响 | 说明 |
|---|---|---|
| `generic/profile.go` | 中 | 新增 StepProfile 类型 |
| `generic/spec.go` BuildRequest/ParseResponse | 大 | 核心改造，支持多步执行 |
| `generic/raw.go` / `rawpacket.go` | 中 | 配置解析支持 steps |
| `generic/inference.go` | 大 | 推理器需识别多步模式 |
| `client/session.go` | 无 | 透传，不感知步骤 |
| 其他 Provider | 无 | 仅 Generic Adapter 需要 |

### 与现有 ChainField 的关系
- **ChainField**：跨轮次，第 N 轮 → 第 N+1 轮（保持不变）
- **StepProfile.ExtractFields**：单轮内，Step N → Step N+1（新增）
- 两者可复用同一个 `ChainField` 结构体，只是作用域不同

### 接入方式：显式声明 vs 自动推理

#### 显式声明（有 API 文档时）
应用层直接配置 Steps 和字段依赖，不需要推理器：
```yaml
steps:
  - name: send_question
    url: https://api.example.com/v1/chat
    body: {"question": "$$$"}
    extract_fields:
      - placeholder: "$$$TASK_ID$$$"
        response_path: "data.task_id"
  - name: get_answer
    url: https://api.example.com/v1/result
    body: {"task_id": "$$$TASK_ID$$$"}
    is_final: true
    text_path: "data.answer"
```
准确率：100%，改造量集中在 GenericSpec 运行时执行逻辑。

#### 自动推理（抓包接入时）
现有推理器核心算法可复用：

| 已有能力 | 函数 | 行号 | 说明 |
|---|---|---|---|
| 值精确匹配 | `checkChainPattern()` | 507-545 | `round[i-1].resp == round[i].req` → 0.85 置信度 |
| 命名语义分析 | `scoreDynamicCandidate()` | 562-594 | `task_id`, `msg_id` 等关键字加分 |
| 首轮空值检测 | `checkSessionIDPattern()` | 448-451 | 强信号 |
| 置信度聚合 | `buildReport()` | 1006-1065 | 几何均值，自动确认阈值 0.85 |

扩展改动点：

| 函数 | 改动 |
|---|---|
| `parseRounds()` 行 135 | 支持每轮内多步 `[]StepPair` |
| `checkChainPattern()` 行 507 | 新增同轮内 `step[j-1].resp → step[j].req` 匹配 |
| `analyzeFields()` 行 224 | 嵌套循环：轮次 × 步骤 |
| `buildProfileFromFields()` 行 839 | 生成按步骤分组的链路规则 |

准确率预估：

| 条件 | 准确率 |
|---|---|
| 2 组样本 | ~70-75% |
| 3 组样本 | ~85-90% |
| 推理 + 用户确认（低于 0.85 阈值时提示） | ~95%+ |

### 关键决策点（实施时确认）
1. **流式支持**：多步中只有最后一步需要流式，还是中间步骤也可能是流式
2. **错误处理**：某个中间步骤失败时，是否需要回滚/重试
3. **超时分配**：总超时如何在各步骤间分配
4. **轮询模式**：是否需要支持"调接口 A → 轮询接口 B 直到完成"的异步任务模式
5. **步骤分组**：抓包场景下靠时间戳间隔还是 URL 差异来区分步骤

### 结论
**可行性：中-高**。
- 有 API 文档 → 显式声明，改造量小，仅改 GenericSpec 运行时逻辑
- 抓包接入 → 自动推理，核心算法可复用，主要新增步骤分组和同轮内值追溯
- 推荐**显式声明优先 + 推理兜底**组合，降低实施风险

---

## 总结

| 场景 | 可行性 | 改造范围 | 对现有 API 的影响 |
|---|---|---|---|
| 多模态 | 高 | 全局（Message + 各 Provider） | 零破坏（并存方案） |
| 单轮多步编排 | 中-高 | 局部（仅 Generic Adapter） | 零破坏（新增 Steps） |

两个能力可以独立实施，互不依赖。
