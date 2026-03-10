# SFRD-TS-03-1.4_V2.0_Generic Adapter通用接入模块微型设计说明书

## 目录

- [1. 介绍](#1-介绍)
  - [1.1. 目的](#11-目的)
  - [1.2. 范围、非目标与假设](#12-范围非目标与假设)
  - [1.3. 定义和缩写](#13-定义和缩写)
  - [1.4. 参考和引用](#14-参考和引用)
  - [1.5. 需求追溯](#15-需求追溯)
- [2. 模块方案概述](#2-模块方案概述)
  - [2.1. 问题与背景](#21-问题与背景)
  - [2.2. 方案概述与架构图](#22-方案概述与架构图)
  - [2.3. 方案选型](#23-方案选型)
  - [2.4. 设计决策记录](#24-设计决策记录)
- [3. 模块详细设计](#3-模块详细设计)
  - [3.1. 数据库设计](#31-数据库设计)
  - [3.2. 接口设计总览](#32-接口设计总览)
  - [3.3. 会话标识模型子模块](#33-会话标识模型子模块)
  - [3.4. Session 执行引擎子模块](#34-session-执行引擎子模块)
  - [3.5. Generic Adapter 子模块](#35-generic-adapter-子模块)
  - [3.6. 原始报文解析子模块](#36-原始报文解析子模块)
  - [3.7. 鉴权抽取子模块](#37-鉴权抽取子模块)
  - [3.8. 存储与归档子模块](#38-存储与归档子模块)
- [4. 质量目标与非功能需求](#4-质量目标与非功能需求)
- [5. 关联分析](#5-关联分析)
- [6. 可靠性设计 (FMEA)](#6-可靠性设计-fmea)
- [7. 部署与回滚策略](#7-部署与回滚策略)
- [8. 验证策略](#8-验证策略)
- [9. 变更控制](#9-变更控制)
  - [9.1. 变更列表](#91-变更列表)
- [10. 修订记录](#10-修订记录)

---

# 1. 介绍

## 1.1. 目的

本说明书描述 Generic Adapter 的重构式设计方案，目标是在不依赖自动识别的前提下，统一标准 API 与非标准 API 的接入路径，并彻底重构多轮对话的会话模型。V2.6 方案在非标准接入上仍依赖"单轮样本+人工占位"配置，接入成本高、调试链路长，V2.7 在此基础上补充 MultiRoundSpec（2~5轮，默认建议3~5轮）自动推断能力。

**核心目标**：
- 取消"本地会话 ID 与远端会话 ID 混用"的实现方式
- 用户必须显式声明目标模型是否支持远端 `session_id` 能力
- 非标准接口只需传 `应用 URL + 请求包 + 响应包 (+ 会话模式)`，SDK 内部自动解析执行
- 新增 MultiRoundSpec（2~5轮请求/响应原始报文，默认建议3~5轮）自动推断，降低"单轮样本+人工占位"接入成本
- 本地始终保存完整对话归档；是否用于请求由会话模式决定

本版本明确为重构方案，不考虑旧行为兼容。

**目标受众**：SDK 开发人员、架构师、测试人员、平台接入工程师。

## 1.2. 范围、非目标与假设

| 类别 | 内容 |
|------|------|
| **范围** | 会话标识模型重构；Session 执行引擎统一；Generic Adapter 模板驱动接入；原始报文（Raw Spec）解析与编译；鉴权标准化提取；SessionStore 归档 |
| **非目标** | 不改动已有 Provider 的上层业务逻辑；不处理 SDK 以外的任务调度与攻击策略；不延续旧 `SessionID` 单字段行为 |
| **前置假设** | 调用方已正确配置目标模型的 URL 和鉴权信息；SessionStore 实现由接入方提供，SDK 仅定义接口；目标模型 API 返回 HTTP 200 表示成功 |

## 1.3. 定义和缩写

| 术语 | 定义 |
|:---|:---|
| SessionID | 会话标识，兼作本地归档查询键与远端会话传递键；来源取决于会话模式 |
| ConversationMode | 会话模式，取值 `remote_session` 或 `local_history` |
| remote_session | 目标模型支持 session_id：SessionID 从首轮响应中提取，后续轮次原样传回目标模型；本地存储仅做归档 |
| local_history | 目标模型不支持 session_id：SessionID 由应用端生成/传入，用于查询本地历史并拼接到每轮请求；本地存储有实用意义 |
| Generic Profile | Generic Adapter 的运行时配置对象 |
| Raw Spec | 外部传入的原始接入描述（URL + 请求包 + 响应包） |
| OnError 策略 | 多轮对话中单轮失败的处理策略：`continue`（跳过继续）或 `abort`（终止任务） |
| HistoryWindow | local_history 模式下历史消息的裁剪窗口配置 |

## 1.4. 参考和引用

1. 统一请求结构：`provider/base/types.go`
2. Session 现状实现：`client/session.go`
3. Provider 接口与注册：`provider/base/spec.go`、`provider/base/registry.go`
4. 流式解析能力：`provider/streaming/sse.go`、`provider/streaming/json_path.go`
5. 配置模型：`config/config.go`
6. 旧项目多轮对话实现参考：`AI-Security-Platform/api/routers/v1/multi_round/handler.go`

## 1.5. 需求追溯

| 需求 ID | 需求描述 | 对应设计章节 | 对应验证用例 |
|---------|---------|-------------|-------------|
| REQ-001 | 用户显式声明会话模式（remote_session/local_history） | 3.3 会话标识模型 | TC-001 |
| REQ-002 | remote_session：SessionID 从目标模型响应提取，后续轮次原样传回目标模型 | 3.3 会话标识模型 | TC-002 |
| REQ-002B | local_history：SessionID 由应用端生成，用于查询本地历史拼接上下文 | 3.3 会话标识模型、3.4 Session 执行引擎 | TC-002B |
| REQ-003 | 非标准 API 通过 RawSpec 三字段接入 | 3.6 原始报文解析 | TC-003 |
| REQ-004 | 本地始终保存完整对话归档 | 3.8 存储与归档 | TC-004 |
| REQ-005 | 鉴权信息从请求头标准化提取 | 3.7 鉴权抽取 | TC-005 |
| REQ-006 | 模板渲染时 input 占位符必须做 JSON 字符串转义 | 3.5 Generic Adapter | TC-006 |
| REQ-007 | 流式响应支持 SSE 格式（data: 前缀、[DONE] 终止、空行跳过） | 3.5 Generic Adapter | TC-007 |
| REQ-008 | HTTP 响应自动处理 gzip 压缩 | 3.5 Generic Adapter | TC-008 |
| REQ-009 | 多轮错误策略可配置（continue/abort） | 3.4 Session 执行引擎 | TC-009 |
| REQ-010 | local_history 模式支持历史窗口裁剪 | 3.4 Session 执行引擎 | TC-010 |
| REQ-011 | 支持多轮请求-响应字段链路传递（ChainFields）：从指定 SSE event 类型提取任意响应字段，注入到下一轮请求体占位符；ChainFields 不传则不启用，传了则 `Placeholder` 与 `ResponsePath` 必填，`ExtractOnEvent` 可空（空表示按任意事件帧提取） | 3.5 Generic Adapter、3.6 原始报文解析 | TC-011 |
| REQ-012 | 支持 MultiRoundSpec 输入（2~5轮，默认建议3~5轮）并自动推断接入配置 | 3.6 原始报文解析 | TC-012 |
| REQ-013 | 自动推断字段分类：`input` / `session_id` / `chain` / `dynamic` / `static` | 3.6 原始报文解析 | TC-013 |
| REQ-014 | 自动推断需输出置信度并支持确认流程（低置信度进入告警态 `pending_confirm`，但不阻断 Session 创建） | 3.6 原始报文解析 | TC-014 |
| REQ-015 | 自动推断失败时必须可回退到手工 RawSpec 模式 | 3.6 原始报文解析 | TC-015 |
| REQ-016 | 自动推断产物需前向兼容 FlowSpec（未来多轮/多步编排） | 3.6 原始报文解析 | TC-016 |

---

# 2. 模块方案概述

## 2.1. 问题与背景

现有实现存在三个结构性问题：

1. `s.id` 同时承担本地存储键和远端会话键，语义冲突，导致 Dify 等 provider 出现特判逻辑。
2. `req.SessionID = s.id` 的默认赋值策略会在会话能力不明时误传远端会话参数。
3. 非标准接入仍依赖"单轮样本+人工占位"：需人工标注 `{{input}}` / `{{session_id}}` / `$$$CHAIN$$$` 占位符，接入成本高且易误配。

此外，对比旧项目（AI-Security-Platform）的实际多轮实现，还暴露出以下工程问题：

- 模板渲染直接拼接用户输入，未做 JSON 转义，输入含引号或换行时产生无效 JSON
- 流式解析未定义 SSE 规范（`data:` 前缀、`[DONE]` 终止），兼容性差
- gzip 响应未处理，目标模型返回压缩内容时解析失败
- 多轮错误策略不一致（部分场景"失败继续"，部分"直接 abort"）
- RemoteSessionID 仅存内存，进程重启后会话丢失

## 2.2. 方案概述与架构图

采用"显式会话模式 + 双 ID 分离 + 统一执行链路"：

- **显式会话模式**：由用户明确传入 `remote_session` 或 `local_history`，SDK 不做自动识别
- **单一 SessionID**：不区分 LocalSessionID / RemoteSessionID，只有一个 `sessionID`，其来源与用途由 mode 决定
- **统一执行**：标准 API 与非标准 API 最终都编译成 `Generic Profile`，由统一 Session 引擎执行

```mermaid
flowchart LR
    A[应用层] --> B{接入输入}
    B -->|标准 API| C[Credential + ProviderConfig]
    B -->|非标准 API| D[Raw Spec]

    D --> E[Raw Parser]
    C --> F[Profile Builder]
    E --> F

    F --> G[Generic Profile]
    G --> H[Session Engine]
    H --> I[Provider Dispatch]
    I --> J[目标模型 API]

    H --> K[SessionStore]
```

## 2.3. 方案选型

| 对比维度 | 方案 A：自动识别远端会话能力 | 方案 B：provider 特判（按平台写死） | 方案 C：显式模式 + 双 ID 分离 |
|---------|--------------------------|----------------------------------|------------------------------|
| 会话正确性 | 中（有误判） | 中 | **高** |
| 接入成本 | 低 | 中 | 中 |
| 维护成本 | 中 | 高 | **低** |
| 风险 | 静默误判 | 每新 provider 须改核心代码 | 接入方需显式配置 mode |
| **结论** | — | — | **选择方案 C**：业务要求"避免误判"，必须由用户显式决定会话模式 |

## 2.4. 设计决策记录

| 决策 ID | 决策内容 | 备选方案 | 决策理由 | 决策人 | 日期 |
|---------|---------|---------|---------|-------|------|
| DR-001 | 不区分首轮/续轮两套请求模板 | 旧项目 firstRound*/followUp* 双模板 | 首轮 SessionID 为空天然不注入 `{{session_id}}`，无需额外模板；双模板增加配置负担 | 徐文彬 | 2026-03-06 |
| DR-002 | OnError 策略作为 Session 级别配置 | 硬编码 abort 或 continue | 旧项目两种攻击方法行为不一致，SDK 层不应绑定业务语义，由调用方决定 | 徐文彬 | 2026-03-06 |
| DR-003 | HistoryWindow 双阈值（条数 + Token） | 仅条数限制 | Token 超限是模型侧的直接失败原因，仅靠条数无法有效控制请求体大小 | 徐文彬 | 2026-03-06 |
| DR-004 | 使用单一 SessionID，不做 Local/Remote 双 ID 分离 | LocalSessionID + RemoteSessionID 两个字段 | remote_session 模式：目标模型返回的 session_id 同时作为本地归档键；local_history 模式：应用端生成的 session_id 用于查询本地历史。两种模式均只需一个 ID，双 ID 分离是过度设计 | 徐文彬 | 2026-03-06 |

---

# 3. 模块详细设计

## 3.1. 数据库设计

无数据库操作。SDK 通过 `SessionStore` 接口抽象存储，具体 Schema 由接入方实现。SDK 定义的 `SessionState` 数据结构见 [3.8 存储与归档子模块](#38-存储与归档子模块)。

---

## 3.2. 接口设计总览

| 序号 | 子模块 | 核心函数签名 | 简要说明 | 详细章节 |
|------|--------|------------|---------|---------|
| 1 | 会话标识模型 | `NewSession(mode, opts...) *Session` | 创建会话，mode 必填 | [3.3](#33-会话标识模型子模块) |
| 2 | Session 执行引擎 | `(s *Session) Chat / ChatStream` | 统一多轮执行入口 | [3.4](#34-session-执行引擎子模块) |
| 3 | Generic Adapter | `BuildRequest / ParseResponse / ParseStreamResponse` | 模板驱动请求构建与响应解析 | [3.5](#35-generic-adapter-子模块) |
| 4 | 原始报文解析 | `ParseRawIntegration(raw) *CompiledIntegration` | Raw Spec 编译为 Profile | [3.6](#36-原始报文解析子模块) |
| 5 | 鉴权抽取 | `ExtractCredential(headers) *Credential` | 从请求头标准化提取鉴权 | [3.7](#37-鉴权抽取子模块) |
| 6 | 存储与归档 | SessionStore interface（Save/Load/Delete） | 会话状态持久化 | [3.8](#38-存储与归档子模块) |

---

## 3.3. 会话标识模型子模块

**功能描述**

定义会话标识与模式语义，SDK 使用单一 `sessionID`，其来源与用途由 `ConversationMode` 决定。

**输入和输出**

- 输入：会话模式、SessionID（remote_session 时可为空待提取；local_history 时由应用端传入）
- 输出：`sessionID`、`ConversationMode`

**数据结构**

```go
type ConversationMode string

const (
    ConversationModeRemoteSession ConversationMode = "remote_session"
    ConversationModeLocalHistory  ConversationMode = "local_history"
)

type Session struct {
    id   string           // 单一 SessionID，含义由 mode 决定
    mode ConversationMode
    // ...
}
```

**两种模式的 SessionID 语义对比**

| 维度 | remote_session | local_history |
|---|---|---|
| **SessionID 来源** | 首轮响应由目标模型返回 | 应用端生成/传入 |
| **发给目标模型** | 是，后续轮次原样传回 | 否，目标模型不支持 |
| **本地存储用途** | 仅归档查询，不用于拼接上下文 | 有实用意义，查询历史后拼入请求 |
| **首轮 SessionID** | 空，不注入请求；从响应提取后持久化 | 已有，直接用于查询本地历史 |

**内部逻辑**

```text
remote_session 模式：
  首轮：SessionID 为空 → 不注入 {{session_id}} → 发出请求
        从响应按 remote_id_path 提取 SessionID → 立即持久化到 SessionStore
  续轮：SessionID 非空 → 注入 {{session_id}} → 发出请求
        本地保存完整对话（归档用）

local_history 模式：
  每轮：按应用端传入的 SessionID 从 SessionStore 查询历史
        历史经 HistoryWindow 裁剪后拼入请求
        SessionID 不注入请求体（目标模型不支持）
        本地保存完整对话（下轮继续使用）
```

**接口设计**

```go
func (s *Session) ID() string
func (s *Session) Mode() ConversationMode
```

**配置项**

| 配置键 | 必填 | 说明 |
|---|---|---|
| `conversation.mode` | 是 | `remote_session` / `local_history` |
| `session.id` | local_history 时是 | 应用端生成的会话 ID；remote_session 时可不传，由首轮响应提取 |

**异常处理**

- 未指定 mode：直接报错，拒绝执行
- `remote_session` 模式首轮响应无法提取 SessionID：报错并提示检查 `remote_id_path` 配置
- `local_history` 模式未传 SessionID：自动生成 UUID 并返回给调用方

---

## 3.4. Session 执行引擎子模块

**功能描述**

统一管理多轮行为，不再根据 provider 名称做特判。支持可配置的错误策略与历史窗口裁剪。

**输入和输出**

- 输入：`ChatRequest`、`ConversationMode`、`OnErrorStrategy`、`HistoryWindow`、`SessionStore` 状态
- 输出：最终发送请求与会话归档结果

**数据结构**

```go
type OnErrorStrategy string

const (
    OnErrorContinue OnErrorStrategy = "continue" // 单轮失败记录后继续下一轮
    OnErrorAbort    OnErrorStrategy = "abort"    // 单轮失败立即终止整个会话
)

type HistoryWindow struct {
    MaxMessages int // 最大保留消息条数，0 表示不限
    MaxTokens   int // 最大 Token 数估算阈值，0 表示不限
}
```

Session 内部还需维护当前轮的链路字段值快照，用于下一轮注入：

```go
// Session 内部字段（非公开）
type Session struct {
    // ...existing fields...
    chainValues map[string]string // 上一轮从响应提取的 ChainField 值，key=占位符（含$$$），val=提取值
}
```

**内部逻辑**

```text
if mode == remote_session:
  - 不加载本地历史（仅发送当前轮输入）
  - s.id != ""（非首轮）时注入 {{session_id}}；首轮 s.id 为空不注入
  - 将 Session.chainValues 作为 req.ChainValues 传入 BuildRequest，供链路占位符替换
  - 从响应提取 SessionID 并立即持久化到 SessionStore
  - 从流式 chunk.ChainValues 按 ExtractOnEvent 累积提取链路字段值；流结束后更新 Session.chainValues（供下一轮使用）
  - 本地照常归档完整对话

if mode == local_history:
  - 从本地加载历史并裁剪（先按 MaxMessages，再按 MaxTokens 估算）
  - 拼接裁剪后的历史发送，不注入 session_id（目标模型不支持）
  - 本地归档完整对话

单轮失败处理（由 OnError 策略决定）：
  OnErrorContinue: 记录错误，继续下一轮
  OnErrorAbort:    立即终止，调用 SessionStore 标记会话状态
```

**流式与非流式的行为差异**

| 行为 | 非流式 Chat() | 流式 ChatStream() |
|---|---|---|
| session_id 注入 | 发请求前按 mode 决定是否注入 | 同左，发请求前处理 |
| session_id 提取 | 收到完整响应后提取并持久化 | goroutine 内收到首个含 SessionID 的 chunk 后**立即持久化**，不等流结束 |
| ChainFields 提取 | 收到完整响应后按 ResponsePath 提取并存入 Session.chainValues | goroutine 内按 ExtractOnEvent 匹配 chunk.ChainValues，流结束后更新 Session.chainValues |
| ChainFields 注入 | 发请求前将 Session.chainValues 传入 req.ChainValues | 同左 |
| 历史保存时机 | 收到完整响应后 `store.Save` | 流全部结束后 `store.Save`（messages 需等全文聚合完毕） |
| 流中断时 session_id | 响应错误时不提取 | session_id 已在首个 chunk 时持久化，中断不丢失 |
| gzip 处理 | 非 200 错误 body 需解压后读取 | 同左（`stream.go:L57`） |

**接口设计**

```go
func (s *Session) Chat(ctx context.Context, req base.ChatRequest) (base.ChatResponse, error)
func (s *Session) ChatStream(ctx context.Context, req base.ChatRequest) (<-chan streaming.StreamChunk, error)
```

**配置项**

| 配置键 | 默认值 | 说明 |
|---|---|---|
| `session.on_error` | `abort` | 单轮失败策略：`continue` / `abort` |
| `session.history_window.max_messages` | `0`（不限） | local_history 模式最大保留消息条数 |
| `session.history_window.max_tokens` | `0`（不限） | local_history 模式 Token 估算上限 |

**异常处理**

- SessionStore 读取失败：降级为无历史（仅 `local_history` 模式生效）
- SessionStore 写入失败：通过 `OnStoreError` 上报，不影响主请求返回
- RemoteID 持久化失败：报错，`remote_session` 模式下不允许静默跳过

---

## 3.5. Generic Adapter 子模块

**功能描述**

将统一请求映射到目标接口并解析响应，支持远端会话字段注入、模板安全渲染、SSE 流式解析和 gzip 响应处理。

**输入和输出**

- 输入：`GenericProfile`、`ChatRequest`、`RemoteSessionID`
- 输出：标准 `ChatResponse` / `StreamChunk`

**数据结构**

```go
// ChainField 描述一条"响应字段 → 下一轮请求字段"的链路传递规则。
// 三个子字段均为必填（当 ChainFields 列表非空时）。
type ChainField struct {
    Placeholder    string // 请求体占位符，格式必须为 $$$NAME$$$，e.g. "$$$PARENT_MSG$$$"
    ResponsePath   string // 响应 JSON 路径，e.g. "message_id"
    ExtractOnEvent string // 指定从哪个 SSE event 类型提取，e.g. "message_end"
}

type GenericProfile struct {
    Request struct {
        Method         string
        Path           string
        BodyTemplate   map[string]any
        SessionIDField string // 例如 session_id / conversation_id
    }
    Response struct {
        TextPath     string
        RemoteIDPath string
        Stream struct {
            Protocol    string     // "sse" | "ndjson"
            DeltaPaths  []string
            DonePath    string
            DoneValue   string
            DoneMarker  string     // SSE 模式下的终止行，如 "[DONE]"
            ChainFields []ChainField // 多轮字段链路传递规则，可为空
        }
    }
    Conversation struct {
        Mode ConversationMode
    }
}
```

**内部逻辑**

```text
模板渲染（REQ-006、REQ-011）：
  - {{input}} 替换前必须做 JSON 字符串转义（等价于 json.Marshal 后去除首尾引号）
    确保用户输入含引号、换行、反斜杠时不破坏 JSON 结构
  - {{session_id}} 仅在 RemoteID 非空时注入，RemoteID 为空时移除该字段
  - ChainFields 占位符（$$$NAME$$$）：
      有值（req.ChainValues[placeholder] != ""）→ 替换为对应值
      无值（首轮或上一轮未提取到）→ 移除该字段，不发送（等同 session_id 首轮行为）
  - {{model}}、{{temperature}}、{{max_tokens}}、{{stream}} 按配置值渲染

HTTP 请求发送：
  - 自动处理 Content-Encoding: gzip 响应（REQ-008）
    resp.Body 使用 gzip.NewReader 包装后再交给后续解析

SSE 流式解析规范（REQ-007，Protocol="sse"）：
  逐行读取响应体：
    1. 跳过空行
    2. 跳过以 "retry:" 开头的行
    3. 剥离 "data: " 前缀（包含末尾空格）
    4. 若剥离后内容等于 DoneMarker（默认 "[DONE]"）→ 结束流
    5. 否则解析 JSON，按 DeltaPaths 提取增量文本

NDJSON 流式解析（Protocol="ndjson"）：
  逐行读取，跳过空行，直接解析 JSON，按 DeltaPaths 提取

远端会话 ID 提取：
  mode=remote_session 时，从每轮响应 JSON 按 RemoteIDPath 提取；
  提取成功后立即调用 SessionStore 持久化

ChainFields 提取（REQ-011）：
  ParseStreamResponse wrapper 对每个 SSE chunk：
    遍历 ChainFields，若 chunk 对应的 SSE event 类型 == ChainField.ExtractOnEvent：
      按 ChainField.ResponsePath 从 chunk.Raw 提取值
      写入 chunk.ChainValues[ChainField.Placeholder]
  session.go 在 goroutine 内累积 chunk.ChainValues，流结束后更新 Session.chainValues
  下一轮 BuildRequest 时将 Session.chainValues 作为 req.ChainValues 传入，用于替换占位符
```

**接口设计**

```go
func (s *GenericSpec) BuildRequest(ctx context.Context, opts base.BuildOptions, req base.ChatRequest) (*http.Request, error)
func (s *GenericSpec) ParseResponse(resp *http.Response) (base.ChatResponse, error)
func (s *GenericSpec) ParseStreamResponse(resp *http.Response) (<-chan streaming.StreamChunk, error)
```

`base.ChatRequest` 新增字段：

```go
type ChatRequest struct {
    // ...existing fields...
    ChainValues map[string]string // 上一轮提取的链路字段值，key=占位符（含$$$），val=提取值
}
```

`streaming.StreamChunk` 新增字段：

```go
type StreamChunk struct {
    // ...existing fields...
    ChainValues map[string]string // 本 chunk 提取到的链路字段值，key=占位符（含$$$），val=提取值
}
```

**配置项**

| 配置键 | 必填 | 说明 |
|---|---|---|
| `generic.request.body_template` | 是 | 请求模板，支持 `{{input}}`、`{{session_id}}`、`$$$NAME$$$` 等占位符 |
| `generic.response.stream.protocol` | 流式时是 | `sse` 或 `ndjson` |
| `generic.response.stream.delta_paths` | 流式时是 | 流式增量文本的 JSON 路径列表 |
| `generic.response.stream.done_marker` | SSE 时是 | SSE 终止标记，默认 `[DONE]` |
| `generic.response.remote_id_path` | remote_session 时是 | 从响应提取远端会话 ID 的 JSON 路径 |
| `generic.response.stream.chain_fields` | 否 | ChainField 列表；传了则每条的三个子字段均必填 |

**异常处理**

- `remote_session` 但未配置 `SessionIDField` 或 `remote_id_path`：初始化时报错，拒绝构建
- 模板占位符渲染后 JSON 非法：返回渲染错误，不发送请求
- ChainFields 列表非空但某条目存在空字段：初始化时报错，拒绝构建
- ChainFields 占位符格式不符合 `$$$NAME$$$`（NAME 非空）：初始化时报错
- gzip 解压失败：返回解压错误并关闭连接
- SSE 行解析 JSON 失败：跳过该行并继续，累计错误超过阈值时终止流并报错
- 非 200 状态码：直接返回 HTTP 错误，不尝试解析 body

---

## 3.6. 原始报文解析子模块

**功能描述**

将外部三字段输入编译为 `GenericProfile + Credential`，URL 解析包含 base_url/path 合成规则；V2.7 新增 MultiRoundSpec（2~5轮原始报文，默认建议3~5轮）自动推断能力，用于替代"单轮样本+人工占位"的高成本接入方式。业务层如仅有 2 轮数据，可直接构造 `MultiRoundSpec{Rounds: []RoundPair{...}}`。

**输入和输出**

- 输入：`RawIntegrationSpec` 或 `MultiRoundSpec`
- 输出：`CompiledIntegration`（手工模式）或 `InferredIntegration`（自动推断模式）

**数据结构**

```go
type RawIntegrationSpec struct {
    URL          string
    Request      RawPacket
    Response     RawPacket
    Conversation struct {
        Mode    ConversationMode // 必填
        OnError OnErrorStrategy  // 默认 abort
    }
    Placeholders struct {
        Input     []string // 默认 ["$$$"]
        SessionID []string // 默认 ["$$$SESSION_ID$$$"]
    }
    // ChainFields 描述多轮字段链路传递规则，可不传。
    // 传了则每条的三个子字段（Placeholder、ResponsePath、ExtractOnEvent）均必填。
    ChainFields []ChainField
}

type RawPacket struct {
    Headers map[string]string
    Body    string
}

type CompiledIntegration struct {
    Credential *auth.Credential
    Provider   *config.ProviderConfig
    Profile    *GenericProfile
}

// MultiRoundSpec 使用 2~5 轮完整原始报文做自动推断（默认建议3~5轮）：
// rounds[i].request + rounds[i].response，i ∈ [1,N]，N ∈ [2,5]
type MultiRoundSpec struct {
    URL    string
    Rounds []struct {
        Request  RawPacket
        Response RawPacket
    }
    Conversation struct {
        Mode    ConversationMode // 必填
        OnError OnErrorStrategy  // 默认 abort
    }
}

type InferredField struct {
    RequestPath  string
    ResponsePath string
    Class        string   // input | session_id | chain | dynamic | static
    Placeholder  string   // {{input}} / {{session_id}} / $$$NAME$$$
    Confidence   float64  // [0,1]
    ConflictWith []string // 发生冲突的候选分类
    Reason       string   // 推断依据摘要
}

type InferenceReport struct {
    OverallConfidence float64
    Status            string // auto_confirmed | pending_confirm | failed
    Fields            []InferredField
    Warnings          []string
    FallbackSuggested bool
    FlowSpecMeta struct {
        Version string // "v1alpha1"
        Source  string // "MultiRoundSpec"
    }
}

// 自动推断输出契约：始终返回 GenericProfile + InferenceReport
type InferredIntegration struct {
    Profile *GenericProfile
    Report  *InferenceReport
}
```

**MultiRoundSpec 输入约束（业务层提供 JSON）**

业务层以独立 JSON 文件提供 2~5 轮样本（默认建议3~5轮），推荐结构如下：

```json
{
  "model": "remote_session",
  "base_url": "https://example.com/v1/chat/completions",
  "rounds": [
    {
      "request": "<raw http request text>",
      "response": "<raw http response text>"
    },
    {
      "request": "<raw http request text>",
      "response": "<raw http response text>"
    }
  ]
}
```

约束规则（Phase-1）：

1. `model` 必填，且只能为 `remote_session` 或 `local_history`。
2. `base_url` 必填。
3. `rounds` 至少 2 轮、最多 5 轮（默认建议3~5轮）；每轮 `request` 与 `response` 均必填，缺一即报错。
4. Phase-1 仅支持可解析 JSON body（请求/响应 body 需可反序列化为 JSON 对象）。
5. `$$$` 仅允许用于“问题文本字段”和“回答文本字段”占位。
6. `session_id` / `parent_message_id` / `tool_call_id` 等链路字段必须保留真实值，不允许替换为 `$$$`。
7. 各轮必须构成真实续轮关系（例如后续轮请求引用前序轮响应中的会话/链路值），不可使用独立会话样本。

**内部逻辑**

```text
URL 合成规则：
  1. 解析 URL 为 scheme + host + path
  2. base_url = scheme://host
  3. path 作为 GenericProfile.Request.Path
  4. 若 URL 缺少 scheme：默认补 https://
  5. 若 URL 为空：返回字段级错误

请求包解析：
  1. 识别输入占位符（默认 $$$）→ 替换为 {{input}}
  2. 识别 session_id 占位符（默认 $$$SESSION_ID$$$）→ 替换为 {{session_id}}
  3. 识别 ChainFields 占位符（$$$NAME$$$，NAME 不为 SESSION_ID）→ 保留原占位符字符串，供 renderTemplate 在运行时替换
  4. 识别 HTTP Method（从请求包 Headers 或默认 POST）
  5. 构造 BodyTemplate

ChainFields 校验：
  若 ChainFields 列表非空，遍历每条：
    Placeholder 为空 → 报错
    Placeholder 格式不符合 $$$NAME$$$ 或 NAME 为空 → 报错
    ResponsePath 为空 → 报错
    ExtractOnEvent 为空 → 允许（按任意事件帧提取）
  通过校验后将 ChainFields 直接写入 GenericProfile.Response.Stream.ChainFields

响应包解析：
  1. 识别文本/delta 路径 → DeltaPaths
  2. 识别 remote_id 路径（含 $$$SESSION_ID$$$ 占位的字段）→ RemoteIDPath
  3. 识别流式协议（有 data: 前缀行 → SSE，否则 → NDJSON）

调用 ExtractCredential 从 Request.Headers 提取鉴权

MultiRoundSpec 自动推断流程（REQ-012~REQ-016）：
  输入为 `rounds`（2~5轮，默认建议3~5轮），每轮包含 request/response 报文。

  1. 预处理（第一阶段：规则推断，deterministic）
    - 将各轮报文 Body 解析为 JSONPath -> 标量值映射
    - 生成跨轮请求 diff：Diff(round[i].request, round[i+1].request)
    - 生成跨轮响应值索引：Index(round[i].response, round[i+1].response)

  2. 字段分类推断规则（规则阶段）
    - input：
      相邻轮请求同一路径值发生变化，且新值不来源于前序轮 response 回写
      -> 该路径标记为 input，模板替换为 {{input}}
    - session_id：
      后续轮 request 某路径值 == 前序轮 response 某路径值，且首轮同路径为空/缺失
      或路径命名命中 session/conversation/thread
      -> 请求路径标记为 session_id，提取路径标记为 RemoteIDPath，模板替换为 {{session_id}}
    - chain：
      后续轮 request 某路径值来源于前序轮 response，且未命中 session_id
      -> 生成 ChainField（Placeholder/ResponsePath/ExtractOnEvent）
    - dynamic：
      请求差异字段无法稳定映射到 input/session_id/chain（如 timestamp/nonce/signature）
      -> 标记为 dynamic，不固化静态值
    - static：
      多轮请求值一致且不依赖响应
      -> 固化到 BodyTemplate

  3. 冲突处理
    - 同一路径命中多个分类时，按置信度最高者生效
    - 若最高分与次高分差值 < conflict_margin（默认 0.10），标记冲突并进入 pending_confirm
    - 冲突优先级（同分时）：session_id > chain > input > dynamic > static
    - 同一 Placeholder 指向多个 ResponsePath：判定为 failed（返回 error，不创建 Session）

  4. AI 协助分析（二阶段判别，可选）
    - AI 协助为可选节点，默认关闭；仅在用户显式开启（配置项或业务参数）时才执行
    - 第一阶段产出冲突字段与低置信字段后，可选调用本地 3B（OpenAI 兼容）模型进行重排与解释增强
    - 未开启 AI 协助时，流程直接采用第一阶段规则推断结果并进入三态状态判定
    - AI 仅可影响“冲突消解建议 / 置信度重排建议 / reason 文本解释”，不能直接覆盖硬约束
    - 硬约束包括：链路字段保真、占位符合法性、必填报文完整性、轮次数范围（2~5）
    - AI 调用失败（不可用、网络异常、超时、重试后失败）不阻断主流程，必须自动降级回第一阶段规则推断结果
    - “80%/90%”仅为示例目标值，实际可用率需由业务回放验证集评估，不构成硬性 SLA。

  5. 置信度与确认流程（告警不阻断）
    - 每个字段输出 confidence（0~1）与 reason
    - 汇总生成 OverallConfidence
    - OverallConfidence >= auto_apply_threshold（默认 0.85）且无冲突 -> auto_confirmed
    - 否则 -> pending_confirm（保留告警与复核建议），仍创建 Session 并返回 InferenceReport

  6. 失败回退与前向兼容
    - 三态会话行为固定：`auto_confirmed` 创建 Session；`pending_confirm` 也创建 Session（保留告警并返回 InferenceReport 供人工复核）；`failed` 返回 error 且不创建 Session
    - 推断失败时，返回 error 且不创建 Session；若 InferenceReport 可用则附带报告用于排障与回退建议，并允许调用方回退到 RawIntegrationSpec 手工模式
    - 输出契约固定为 GenericProfile + InferenceReport
    - Report.FlowSpecMeta 固定写入 Version=v1alpha1、Source=MultiRoundSpec，保证后续扩展到 FlowSpec（N 轮/多步）时接口前向兼容
```

### AI 协助标准 Prompt（OpenAI 兼容）

为保证二阶段 AI 协助可复现、可审计、可降级，约定以下 Prompt 与输出规范。

**System Prompt 模板（角色、边界、禁止项）**

```text
你是 MultiRoundSpec 推断流程中的“二阶段判别助手”。
你的任务仅限：
1) 给出冲突消解建议；
2) 给出置信度重排建议；
3) 生成可审计的 reason 解释。

边界与硬约束：
- 你不得修改或放宽 hard_constraints。
- 你不得输出超出候选集合（rule_candidates）的新分类体系。
- 你不得更改轮次数约束（2~5）、报文完整性、占位符合法性、链路字段保真要求。
- 当证据不足或冲突未解时，必须建议人工确认（needs_human_confirm=true）。

输出要求：
- 仅输出 JSON；不得输出 Markdown、注释、前后缀解释文本。
- 字段必须符合调用方给定 Schema；confidence 范围必须在 [0,1]。
- 若无法可靠判断，保持保守：提高风险标记并要求人工确认。
```

**User Prompt 模板（输入数据结构）**

```text
请基于以下输入执行“冲突消解建议 + 置信度重排建议 + reason 解释”，并严格按 JSON Schema 输出。

输入数据（JSON）：
{
  "rounds": [
    {
      "index": 1,
      "request": {"flat_jsonpath": {"$.a.b": "..."}},
      "response": {"flat_jsonpath": {"$.x.y": "..."}}
    }
  ],
  "rule_candidates": [
    {
      "path": "$.request.body.session_id",
      "candidates": [
        {"classification": "session_id", "confidence": 0.72, "reason": "..."},
        {"classification": "chain", "confidence": 0.66, "reason": "..."}
      ],
      "rule_winner": {"classification": "session_id", "confidence": 0.72}
    }
  ],
  "conflicts": [
    {
      "path": "$.request.body.session_id",
      "type": "close_scores",
      "detail": "top1-top2 < conflict_margin"
    }
  ],
  "hard_constraints": {
    "round_count_range": [2, 5],
    "placeholder_rules": [
      "$$$ only allowed for question/answer text fields",
      "session/chain fields must keep real values"
    ],
    "required_packets": "each round requires request+response",
    "chain_integrity": "no placeholder -> multi response paths"
  },
  "thresholds": {
    "auto_apply_threshold": 0.85,
    "conflict_margin": 0.10
  }
}
```

**输出 JSON Schema（最小字段集）**

```json
{
  "type": "object",
  "additionalProperties": false,
  "required": [
    "needs_human_confirm",
    "field_decisions",
    "conflict_resolutions",
    "overall_confidence",
    "risk_flags"
  ],
  "properties": {
    "needs_human_confirm": {"type": "boolean"},
    "field_decisions": {
      "type": "array",
      "items": {
        "type": "object",
        "additionalProperties": false,
        "required": ["path", "classification", "confidence", "reason"],
        "properties": {
          "path": {"type": "string"},
          "classification": {
            "type": "string",
            "enum": ["input", "session_id", "chain", "dynamic", "static"]
          },
          "confidence": {"type": "number", "minimum": 0, "maximum": 1},
          "reason": {"type": "string"}
        }
      }
    },
    "conflict_resolutions": {
      "type": "array",
      "items": {
        "type": "object",
        "additionalProperties": false,
        "required": ["path", "resolution", "reason"],
        "properties": {
          "path": {"type": "string"},
          "resolution": {"type": "string"},
          "reason": {"type": "string"}
        }
      }
    },
    "overall_confidence": {"type": "number", "minimum": 0, "maximum": 1},
    "risk_flags": {
      "type": "array",
      "items": {"type": "string"}
    }
  }
}
```

**对外结果约定（用户反馈）**

为满足“反馈给用户的结果里必须包含修改建议”，推断接口对外返回应包含 `suggestions` 字段：

- `suggestions`：可执行修改建议列表（可为空，取决于状态）
- 列表每项最小字段：
  - `target`：建议作用目标（字段路径/配置项）
  - `action`：建议动作（replace/add/remove/review 等）
  - `value`：建议写入值或候选值
  - `reason`：建议原因（对应冲突、低置信或失败原因）
  - `priority`：优先级（high/medium/low）

状态约束：

- `pending_confirm`：`suggestions` 必须非空
- `failed`：`suggestions` 至少包含一条“回退手工模式（RawIntegrationSpec）”建议
- `auto_confirmed`：`suggestions` 可为空

用户可读返回示例（JSON）：

```json
{
  "result": "pending_confirm",
  "score": 0.78,
  "summary": "发现 2 个冲突字段，已创建 Session 并标记为 pending_confirm，需人工复核。",
  "can_apply_directly": false,
  "draft_config": {
    "conversation_mode": "remote_session",
    "session_id_path": "$.conversation_id"
  },
  "issues": [
    "$.request.body.session_id 在 session_id 与 chain 间冲突"
  ],
  "suggestions": [
    {
      "target": "$.request.body.session_id",
      "action": "review",
      "value": "{{session_id}}",
      "reason": "首轮为空且续轮回写命中前序响应字段",
      "priority": "high"
    }
  ],
  "next_steps": [
    "确认冲突字段归类",
    "复核冲突字段并更新告警处置结论"
  ]
}
```

**调用失败与降级**

- AI 输出非 JSON、JSON 解析失败、字段缺失、字段越界（如 confidence 不在 [0,1]）时：视为无效输出，统一降级回规则阶段结果。
- AI 服务超时、不可用、网络异常、重试后失败时：不阻断主流程，统一降级回规则阶段结果。
- 降级后仍按规则阶段结果执行状态分流：`auto_confirmed / pending_confirm / failed` 判定逻辑不变。

**接口设计**

```go
func ParseRawIntegration(raw RawIntegrationSpec) (*CompiledIntegration, error)
func InferIntegrationByMultiRound(spec MultiRoundSpec) (*InferredIntegration, error)
func InferIntegrationByMultiRoundWithConfig(spec MultiRoundSpec, cfg InferenceConfig) (*InferredIntegration, error)
func (c *Client) NewSessionFromRaw(raw RawIntegrationSpec, opts ...SessionOption) (*Session, error)
func (c *Client) NewSessionFromMultiRound(spec MultiRoundSpec, opts ...SessionOption) (*Session, *InferredIntegration, error)
func (c *Client) NewSessionFromMultiRoundWithConfig(spec MultiRoundSpec, cfg InferenceConfig, opts ...SessionOption) (*Session, *InferredIntegration, error)
```

调用层可通过可选业务参数控制是否启用 AI 协助；未启用时行为与纯规则推断完全一致。

**配置项**

| 配置键 | 默认值 | 说明 |
|---|---|---|
| `raw.placeholders.input` | `[$$$]` | 输入占位符列表 |
| `raw.placeholders.session_id` | `[$$$SESSION_ID$$$]` | 会话 ID 占位符列表 |
| `raw.inference.auto_apply_threshold` | `0.85` | `auto_confirmed` 判定阈值，低于该值进入 `pending_confirm`（仍创建 Session，附带告警报告） |
| `raw.inference.conflict_margin` | `0.10` | 冲突判定阈值（最高分与次高分差值） |
| `raw.inference.default_extract_event` | `message_end` | Chain 推断缺省的 SSE event 类型 |
| `raw.inference.ai_assist.enabled` | `false` | 是否启用本地 3B（OpenAI 兼容）AI 协助分析 |
| `raw.inference.ai_assist.base_url` | `` | 本地 OpenAI 兼容服务地址（enabled=true 时必填） |
| `raw.inference.ai_assist.api_key` | `` | 本地 OpenAI 兼容服务鉴权密钥（enabled=true 时必填） |
| `raw.inference.ai_assist.model` | `` | 本地 OpenAI 兼容模型名（enabled=true 时必填） |
| `raw.inference.ai_assist.timeout_ms` | `3000` | AI 协助分析单次调用超时时间（毫秒） |
| `raw.inference.ai_assist.max_retries` | `1` | AI 协助分析最大重试次数 |

> 说明：AI 协助为可选能力，默认关闭；仅在用户显式开启（配置项或业务参数）时执行。未开启时直接采用规则推断并进入三态状态判定；开启但 AI 调用失败不阻断主流程，自动回退纯规则推断结果。启用 AI 协助时，`base_url` / `api_key` / `model` 任一缺失视为 AI 不可用，自动降级回规则阶段结果。除 SDK 内置可选 AI 辅助外，也允许业务层在 SDK 外独立执行 AI 复核（SDK 按 `auto_confirmed/pending_confirm/failed` 三态返回，`pending_confirm` 保留告警并返回 InferenceReport 供人工复核）。

**异常处理**

- Raw JSON 不可解析：返回字段级错误（字段路径 + 原因）
- mode 缺失：拒绝构建，返回 `"generic: conversation mode is required"` 错误
- `remote_session` 模式下找不到 session_id 占位：返回 `"raw: session_id placeholder not found in request body"` 错误
- MultiRoundSpec 轮次不在 [2,5] 或任一轮缺失 request/response：返回 `"generic: multi_round_spec requires 2-5 rounds with request/response in each round"` 错误
- MultiRoundSpec 任一 request/response body 不是合法 JSON：返回 `"generic: round[i].request/response body not valid JSON"` 错误
- 占位符使用不合法（`$$$` 用于非问题/回答字段，或链路字段被替换为 `$$$`）：返回 `"raw: invalid placeholder usage"` 错误
- 自动推断存在冲突或低置信度：返回 `pending_confirm` 报告，创建 Session 并保留告警与复核建议
- 自动推断失败：返回 error（状态 `failed`）且不创建 Session；若 InferenceReport 可用则附带报告用于排障与回退建议，并提示回退 RawIntegrationSpec 手工模式
- AI 协助不可用：返回 `"raw: ai assist unavailable"` 可降级告警，不直接失败（含启用 AI 协助时 `base_url` / `api_key` / `model` 任一缺失场景）
- AI 协助超时：返回 `"raw: ai assist timeout"` 可降级告警，不直接失败
- AI 协助重试后失败：返回 `"raw: ai assist retries exhausted"` 可降级告警，不直接失败
- URL 为空或无法解析：返回字段级错误

---

## 3.7. 鉴权抽取子模块

**功能描述**

从原始请求头抽取并标准化鉴权配置，处理冲突规则与无鉴权场景。

**输入和输出**

- 输入：`RawPacket.Headers`（map[string]string）
- 输出：`*auth.Credential`

**内部逻辑**

```text
优先级（从高到低）：
  1. Authorization: Bearer xxx → bearer_token 类型
  2. Authorization: Basic xxx → basic_auth 类型
  3. Cookie: xxx             → none 类型 + 自定义 headers
  4. X-* 自定义头            → none 类型 + 自定义 headers
  5. query 参数中的 token/key → query_params 类型

冲突判断：
  若同时命中 1/2 和 3/4，视为冲突 → 按优先级取最高级，其余作为额外 headers 附带
  若同时命中 bearer 和 basic → 直接报错，要求调用方明确

脱敏要求：
  Authorization / Cookie 字段在日志中必须 mask，仅保留前 4 字符 + "****"
```

**接口设计**

```go
func ExtractCredential(headers map[string]string) (*auth.Credential, error)
```

**异常处理**

- 同时存在多个冲突的主认证头（Bearer + Basic）：报错，不自动猜测
- 鉴权头为空：允许（无鉴权内网场景），返回 `none` 类型 Credential

---

## 3.8. 存储与归档子模块

**功能描述**

始终保存完整对话归档，并持久化 local/remote 会话映射，支持进程重启后会话恢复。

**输入和输出**

- 输入：用户消息、模型回复、Session 标识（LocalID + RemoteID）
- 输出：`SessionState`

**数据结构**

```go
type SessionState struct {
    ID        string            // SessionID（remote_session 下为目标模型返回值；local_history 下为应用端生成值）
    Provider  string
    Messages  []Message
    Meta      map[string]string // 包含 mode、on_error
    UpdatedAt time.Time
}
```

**内部逻辑**

```text
请求前：
  - local_history 模式：从 SessionStore 按 SessionID 加载历史，应用 HistoryWindow 裁剪
  - remote_session 模式：SessionID 已持久化，直接从 Session 对象取值注入请求

响应后：
  - 追加 assistant 消息到 Messages
  - remote_session 模式：若 SessionID 为空（首轮），从响应提取并更新 Session.id，立即持久化
  - 调用 SessionStore.Save 持久化（含完整 Messages）

Save 失败处理：
  - 通过 OnStoreError 回调上报错误
  - remote_session 首轮 Save 失败（SessionID 丢失）：视为严重错误，按 OnError 策略处理
  - local_history Save 失败：记录错误，继续返回主响应
```

**异常处理**

- Save 失败（local_history）：记录错误，继续返回主响应
- Save 失败（remote_session）：按 OnError 策略处理，不允许静默丢失 RemoteID

---

# 4. 质量目标与非功能需求

| 指标 | 目标值 | 依据 | 监测方式 |
|------|-------|------|---------|
| Chat 接口额外延迟（SDK 自身开销） | ≤ 5ms（P99） | SDK 不应成为瓶颈 | 基准测试 |
| 流式解析吞吐 | ≥ 10MB/s | 目标模型典型响应速率 | 基准测试 |
| SessionStore 故障降级 | 不影响主请求返回 | 可用性要求 | 单元测试模拟故障 |
| 内存占用（local_history，HistoryWindow=0） | 随历史线性增长，需由调用方配置窗口 | 性能风险说明 | 内存 profile |
| 安全合规 | Authorization/Cookie 日志必须 mask | 安全规范 | 日志审查 |

---

# 5. 关联分析

> SDK 提供“MultiRound（2~5轮，默认建议3~5轮）输入 -> 最终 `request_json.json` 生成能力”，并按 `auto_confirmed/pending_confirm/failed` 三态返回结果；其中 `pending_confirm` 仍创建 Session 并返回告警报告。

### 当前代码关联影响点清单

> 代码扫描基准：V2.1 设计方案，含 6 个补充点的影响范围核查。标注 🆕 的条目为本次新增。

| 影响域 | 当前代码位置 | 当前行为 | 重构影响点 | 处理动作 |
|---|---|---|---|---|
| 会话数据模型 | `provider/base/types.go` | `ChatRequest.SessionID` / `ChatResponse.SessionID` 混用语义 | 需明确 SessionID 在两种 mode 下的语义（来源/是否传给目标模型） | 字段保留 `SessionID` 命名，通过 mode 区分其语义；同步全链路调用 |
| 流式分片模型 | `provider/streaming/types.go` | `StreamChunk.SessionID` 表达不明确 | remote_session 模式下流式回传需携带目标模型返回的 SessionID | 保留字段名，明确语义为目标模型返回的会话 ID，更新聚合器 |
| Session 引擎核心 | `client/session.go`（L97、L107、L122、L152、L158、L186、L200、L210、L225、L264、L288、L312、L330） | **Chat()非流式**：步骤4无条件 `req.SessionID = s.id`（本地 UUID 被当成远端 session_id 发出）；步骤5仅靠 `s.id == ""` 隐式触发提取，且不持久化。**ChatStream()流式**：goroutine 内提取到 SessionID 后仅更新内存（L265-273），流全部结束后才 Save（L288-308）；流中断时 `streamErr != nil` 直接 return 不保存（L284-286）；`isDifyProvider()` 特判控制两处 SessionID 生成时机（L97、L200、L330）；历史全量加载无裁剪 | 根因直接体现：非 Dify 本地 UUID 被当成远端 session_id 误传；流中断时 SessionID 仅存内存，进程重启丢失；新设计通过 ConversationMode 将隐式分支变为显式配置 | **Chat()** 改为：`remote_session` 且 `s.id != ""`（非首轮）才注入 session_id；`remote_session` 每轮提取并持久化；`local_history` 加载历史 + HistoryWindow 裁剪，不注入 session_id。**ChatStream()** 额外改为：`remote_session` 提取到 SessionID 后**立即** `store.Save` 持久化，不等流结束；流中断时已提取的 session_id 已持久化。删除 `isDifyProvider()` 及 `isDifyName()` 函数 |
| Session 配置 | `client/session_config.go`（L10、L15） | 仅有 `OnStoreError` 回调；无 `OnErrorStrategy` 类型 | 多轮错误策略入口缺失 | 新增 `OnErrorStrategy`（continue/abort）字段；新增 `HistoryWindow`（MaxMessages/MaxTokens）配置入口 |
| Session 构造入口 | `client/client.go`（L102）；`client/test.go`（L54、L86） | `NewSession/NewSessionWith` 不要求会话模式；`Test()`/`TestWith()` 使用 `WithHistoryMode(HistoryNone)` + `WithStore(nil)` 表示单轮无状态请求 | mode 必填校验不应在构造时强制触发；连通性测试是单轮一次性请求，`ConversationMode` 对其无意义 | mode 校验改为**惰性校验**（第二轮发起时才检查）；或在 Session 内部对 `WithStore(nil)` 场景豁免 mode 校验；`Test()` 无需修改 |
| 流式结果聚合 | `client/stream.go`（L56、L95、L104、L111） | `collectStream` 聚合 SessionID 来自 chunk（L104-106）；错误响应 body 直读无 gzip 处理（L57）；`collectStream` 仅聚合不持久化，持久化由 `ChatStream()` goroutine 负责 | `collectStream` 供内部 `chatWithStreamSync` 使用，不涉及 Session 持久化；但 gzip 未处理会导致非 200 错误体乱码 | 错误 body 读取（L57）加 gzip 解压；`collectStream` 本身无需改动（持久化逻辑在 Session 层）|
| 🆕 SSE 解析规范 | `provider/streaming/sse.go`（L85、L86、L94、L121） | 已剥离 `data:`、识别 `[DONE]`、空行聚合；**未显式跳过 `retry:` 行**；未知行静默忽略 | 需补 `retry:` 跳过；统一明确 SSE 行处理规范 | 补 `retry:` 行判断；完善单元测试覆盖所有 SSE 行类型 |
| 🆕 SSE 调用方（各 Provider 流式） | `provider/impls/openai/compat_stream.go`（L22）；`provider/impls/claude/stream.go`（L25）；`provider/impls/gemini/stream.go`（L23）；`provider/impls/dify/stream.go`（L87、L108） | 直接依赖 SSEParser；Dify 有独立 SSE 处理同样缺 `retry:` | SSE 规范变更需各 Provider 回归验证；Dify 独立实现需同步修改 | 各 Provider 补回归测试；Dify `stream.go` 补 `retry:` 跳过 |
| 🆕 gzip 解压（非流式） | `provider/impls/claude/spec.go`（L78）；`provider/impls/openai/compat.go`（L81）；`provider/impls/gemini/spec.go`（L85）；`provider/impls/ollama/spec.go`（L58）；`provider/impls/dify/spec.go`（L95）；`provider/impls/plugin/spec.go`（L73） | 各处 `io.ReadAll` 直读 `resp.Body`，无 gzip 处理 | 目标模型返回 gzip 压缩响应时解析完全失败 | 抽取通用 `readResponseBody` 函数统一处理 `Content-Encoding: gzip` |
| 🆕 gzip 解压（流式） | `provider/streaming/sse.go`（L39）；`provider/streaming/ndjson.go`（L37）；`provider/impls/dify/stream.go`（L33） | 直接读 `resp.Body`，无 gzip 处理 | 流式 gzip 响应解析失败 | 在 parser 入口前统一检测并包装 gzip.Reader |
| 🆕 JSON 模板渲染（新增模块） | 当前代码库中不存在 | 所有 Provider 均用结构化 payload + `json.Marshal`，无 `{{input}}` 占位符渲染逻辑 | Generic Adapter 需新增模板渲染能力；`{{input}}` 替换前必须做 JSON 字符串转义 | 新增模板渲染 helper；转义规则：等价于 `json.Marshal(input)` 后去除首尾引号；单元测试覆盖引号、换行、反斜杠等特殊字符 |
| 🆕 本地 OpenAI 兼容 AI 协助推断 | `provider/impls/generic/rawpacket.go`、`provider/impls/generic/raw.go`、`config/config.go` | 当前仅规则推断，无 AI 二阶段判别 | 新增本地 3B（OpenAI 兼容）协助冲突字段/低置信字段重排与解释；该能力可由 SDK 内置开关控制，或由业务层在 SDK 外独立实现；两种方式都必须满足审计可追踪与失败可降级 | 增加 `raw.inference.ai_assist.*` 配置；调用失败仅告警并回退规则结果；记录 AI 参与痕迹与降级原因用于审计 |
| 🆕 HistoryWindow 裁剪 | `session/types.go`（L35）；`session/truncate.go`（L27） | `TruncatePolicy` 定义存在但从未被 Session 调用；`MaxTokens` 仅在 `GetOptions` 中定义未落地 | 历史全量加载，local_history 长会话下请求体无限增长 | 落地 `TruncatePolicy` 调用；扩展为 MaxMessages + MaxTokens 双阈值；在 Session 加载历史时裁剪 |
| Dify Provider | `provider/impls/dify/spec.go`、`stream.go` | 通过 `req.SessionID` / `conversation_id` 传递会话 | 需适配远端会话新字段与 mode 约束；同步 gzip 与 SSE retry: 修复 | 显式使用远端会话字段；`local_history` 模式不传 `conversation_id`；同步 gzip/SSE 修复 |
| Plugin Provider | `provider/impls/plugin/spec.go` | `session_id` 与 `startNewChat` 透传 | 需与新会话字段对齐；同步 gzip 修复 | 按 mode 控制是否透传；同步 gzip 修复 |
| 配置模型 | `config/config.go` | 无 `conversation.mode`、`OnErrorStrategy`、`HistoryWindow`、generic profile 结构 | 无法承载显式模式与新配置项 | 扩展 ProviderConfig：新增会话模式、OnErrorStrategy、HistoryWindow、generic 配置结构 |
| 文档体系 | `docs/session-guide.md` 等 | 建立在旧 `SessionID` 语义上 | 与重构设计不一致 | 全量改写会话章节；补充 JSON 转义、SSE 规范、gzip、HistoryWindow 说明 |
| 示例代码 | `examples/`（多轮与插件示例） | `WithID` 默认等同请求会话 ID | 行为变化后会误导 | 按新接口改造，显式设置 mode；补充 OnError 与 HistoryWindow 配置示例 |
| 测试基线 | `test/client_test.go` 等 | 断言基于旧 `SessionID` 行为 | 重构后系统性失败 | 按新语义重写断言矩阵（remote/local 两模式）；补充 gzip、SSE、HistoryWindow、OnError 场景 |

### V2.7 对当前 SDK 的关联改动补充清单

> 目标：把 V2.7 MultiRoundSpec（2~5轮，默认建议3~5轮）设计与当前代码实现逐项对齐。以下为最小改动集，按文件维度列出“当前状态 / 需要改动 / 风险点 / 验证点”。

| 文件 | 当前状态 | 需要改动 | 风险点 | 验证点 |
|---|---|---|---|---|
| `client/client.go` | 已有 `NewSessionFromHTTPSpec`、`NewSessionFromRaw`，并已提供 `NewSessionFromMultiRound` / `NewSessionFromMultiRoundWithConfig` 主入口 | 保持推断入口按报告状态分流：`auto_confirmed` 创建 Session，`pending_confirm` 也创建 Session（保留告警并返回 `InferenceReport` 供人工复核），`failed` 返回错误且不创建 Session | 若静默放行低置信度结果且缺少告警复核信息，会把错误映射带到运行时 | 构造三类样例：auto_confirmed / pending_confirm / failed；分别断言返回契约与建会话行为 |
| `client/session.go` | 已分 `remote_session/local_history/legacy`；流式首包已即时持久化 SessionID；已维护 `chainValues` | 增加“推断确认态”接入位（至少在 `SessionState.Meta` 中标识 inference_status/source/version）；`auto_confirmed` 与 `pending_confirm` 均可进入自动注入链路（`pending_confirm` 仅附带告警与复核信息）；保留手工回退路径 | 若确认态与运行态不一致，可能出现同一 session 配置漂移 | 流式中断恢复、重启恢复、pending 告警可追踪、后续轮一致性回归；校验 `Meta` 状态可追踪 |
| `provider/impls/generic/raw.go` | 已有 `ParseRawIntegration`、占位符转换、鉴权提取；`validateChainFields` 当前仅强制 `placeholder/response_path`，`extract_on_event` 可空 | 保持 `ExtractOnEvent` 可空语义并在文档统一口径；自动推断统一只走 MultiRoundSpec（2 轮样本由业务层直接组装为 MultiRoundSpec） | 文档若继续写“必填”会与实现断层，导致接入误判为错误配置 | `extract_on_event=""` 样例（DeepSeek/OneAPI）可正常通过；非空时按 event 精准提取 |
| `provider/impls/generic/rawpacket.go` | 已具备 HTTP 报文解析、SSE/NDJSON 自动识别、delta/remote_id 占位推断；终止事件关键词未覆盖 `close` | 新增 MultiRoundSpec（2~5轮，默认建议3~5轮）跨轮 diff 推断核心；字段分类（input/session_id/chain/dynamic/static）；冲突与置信度报告；终止事件识别补 `close` | `close` 未识别会导致 done 判定偏差；轮次样本不足时易低置信度误判 | DeepSeek `event: close` 样例回归；冲突样例进入 `pending_confirm`；缺包样例返回结构化错误 |
| `provider/impls/generic/profile.go` | 现有 GenericProfile/StreamProfile 可承载运行时配置；无推断报告结构 | 增加 `InferredField` / `InferenceReport` / `InferredIntegration`（或等价结构）定义，固定输出契约 | 若报告结构不稳定，后续 FlowSpec 扩展会破坏兼容 | 序列化/反序列化一致性；`FlowSpecMeta{Version=v1alpha1,Source=MultiRoundSpec}` 固定输出 |
| `provider/impls/generic/spec.go` | 已支持模板渲染、gzip、流式链路提取；`ExtractOnEvent` 为空时按“任意帧提取” | 保持与 `raw.go/rawpacket.go` 一致的事件匹配语义；确保 pending 场景按“创建 Session + 保留告警 + 返回 `InferenceReport`”运行，不做阻断式分支 | 若 pending 场景缺少告警与复核信息，可能导致低置信推断被误用 | `auto_confirmed`/`pending_confirm` 输入端到端均可用并返回对应报告；`failed` 返回错误且不创建 Session |
| `provider/base/types.go` | `ChatRequest` 已有 `ChainValues`，`ChatResponse`/`StreamChunk` 可承接会话与链路值 | 最小增量增加可选推断元信息字段（如 inference status/source）或约定走 `Meta`；避免破坏既有 provider 接口 | 直接改核心结构若不兼容，影响全部 provider | 全 provider 编译通过；非 generic provider 行为零变更 |
| `config/config.go` | `ProviderConfig.GenericProfile map[string]any` 可承载 generic 配置；无 typed inference 配置 | 增加 `raw.inference` 对应配置承载（阈值、conflict_margin、默认事件）；支持默认值与覆盖 | 配置入口缺失会导致阈值写死、不可审计 | 配置加载回归：默认值生效、显式覆盖生效、非法值报错 |
| `examples/06-generic-raw/*` | 现有样例覆盖 remote/local；已出现 `extract_on_event:""` 与 `event: close` 真实样本 | 新增 MultiRoundSpec 样例（2~5轮，默认建议3~5轮）；补 pending_confirm 的“创建 Session + 告警复核”演示与手工回退演示；补 2 轮输入直接构造 MultiRoundSpec 的示例 | 示例与实现不一致会误导接入方 | 示例可直接跑通：auto_confirmed 一键接入；pending_confirm 创建 Session 且返回告警报告供人工复核；failed 回退路径可执行 |

### 分阶段落地建议（Phase-1 / Phase-2）

| 阶段 | 目标 | 范围 | 交付物 | 验收基线 |
|---|---|---|---|---|
| Phase-1 | 先打通 MultiRoundSpec（2~5轮，默认建议3~5轮）到可用报告链路，明确三态会话行为 | 实现多轮推断、字段分类、冲突检测、置信度计算；输出 `InferenceReport`；状态仅允许 `auto_confirmed/pending_confirm/failed`；其中 `auto_confirmed` 与 `pending_confirm` 均创建 Session（`pending_confirm` 保留告警并返回报告供人工复核），`failed` 返回错误且不创建 Session；提供 RawSpec 手工回退 | `InferIntegrationByMultiRound` + `InferIntegrationByMultiRoundWithConfig` + `NewSessionFromMultiRound` + `NewSessionFromMultiRoundWithConfig` + 示例与测试矩阵 | 三类样例分别满足：auto_confirmed 创建 Session；pending_confirm 创建 Session 且返回告警报告；failed 返回错误且可回退手工模式；不引入破坏性接口变更 |
| Phase-2 | 再做确认持久化与告警治理 | 增加确认结果持久化（SessionStore/配置层）；补充审计字段（确认人、版本、时间）与失效策略；保持 Phase-1 三态建会话行为不变 | 确认态存储模型 + 告警治理策略 + 回滚/失效机制 | 重启后确认态可恢复；告警处置可追踪可回滚；审计链完整；`REQ-014 state（用户确认后再次加载同一推断结果）` 作为 Phase-2 验收项通过 |

### 本轮可行性复审结论（V2.7）

- 结论：**设计可行，但需先补齐“入口分流 + 报告承载 + 文档口径一致性”三处断层。**
- 关键断层 1：`MultiRoundSpec -> Session` 主入口已落到 `client/client.go`；后续重点是三态治理（`auto_confirmed/pending_confirm/failed`）的持久化与审计闭环。
- 关键断层 2：`InferenceReport` 尚无稳定承载结构（当前 profile/spec 仅覆盖运行态）。
- 关键断层 3：文档中“ExtractOnEvent 必填”与当前实现（可空）不一致；建议以实现与真实样例为准，统一为“可空，空表示任意事件帧提取”。
- 兼容性提醒：`rawpacket.go` 的终止事件识别建议补 `close`，以覆盖 DeepSeek 实际 `event: close`。

### 重构顺序建议

1. 数据结构与 Session 核心（`base/types`、`client/session`、`client/stream`）
2. Provider 适配与 Raw Parser（dify / plugin / generic）
3. 测试、示例、文档

### 性能影响

- `local_history` 模式下请求包随历史增长，**必须配置 HistoryWindow**，否则存在请求体过大的风险
- `remote_session` 模式网络负载更小，但依赖远端会话稳定性

### 安全性

- 原始报文解析不信任输入，需做 JSON 和 Header 注入校验
- 日志必须脱敏（Authorization / Cookie），仅保留前 4 字符 + `****`

---

# 6. 可靠性设计 (FMEA)

**评分标准**：

| 评分项 | 1-3（低） | 4-6（中） | 7-10（高） |
|-------|----------|----------|-----------|
| **S 严重度** | 用户无感知或可自动恢复 | 部分功能不可用，有降级方案 | 核心业务中断，数据丢失 |
| **O 发生概率** | 极少（年均 < 1次） | 偶尔（月均 1-3次） | 频繁（周均 > 1次） |
| **D 检测难度** | 现有监控可立即发现 | 需人工排查或延迟发现 | 无监控，依赖用户反馈 |

RPN = S × O × D，≥200 为 High（上线前必须完成），100-199 为 Medium，<100 为 Low。

| 失效模式 | 失效影响 | 失效原因 | S | O | D | RPN | 优先级 | 改进措施 | 责任人 | 完成时间 | 状态 |
|:---|:---|:---|:---:|:---:|:---:|:---:|:---:|:---|:---|:---|:---|
| remote_session 无远端会话 ID | 多轮会话中断 | 响应映射缺失或服务端未返回 | 8 | 4 | 4 | 128 | Medium | remote_session 模式强制配置 `remote_id_path`，缺失即初始化失败 | SDK开发者 | V2.0 | 待实现 |
| local_history 历史无限增长 | 延迟升高 / 请求失败 | 未配置 HistoryWindow | 7 | 6 | 3 | 126 | Medium | 引入 HistoryWindow 双阈值（条数 + Token 估算）；默认值警告提示 | SDK开发者 | V2.0 | 待实现 |
| 模板渲染产生无效 JSON | 目标模型解析失败，请求必然报错 | input 含引号/换行未转义 | 8 | 5 | 4 | 160 | Medium | 渲染 {{input}} 前强制 JSON 字符串转义；单元测试覆盖特殊字符场景 | SDK开发者 | V2.0 | 待实现 |
| SSE [DONE] / data: 前缀未处理 | 流式解析乱码或解析失败 | SSE 规范理解不完整 | 7 | 6 | 3 | 126 | Medium | 明确 SSE 解析规范：剥离 data: 前缀、识别 [DONE]、跳过空行和 retry: 行 | SDK开发者 | V2.0 | 待实现 |
| gzip 响应未解压 | 响应 body 乱码，解析完全失败 | 未检查 Content-Encoding | 6 | 4 | 3 | 72 | Low | HTTP 层自动检测 Content-Encoding: gzip 并解压 | SDK开发者 | V2.0 | 待实现 |
| RemoteID 进程重启后丢失 | 续轮变成新会话，上下文断裂 | 仅存内存未落库 | 8 | 5 | 4 | 160 | Medium | 每轮响应后立即持久化 RemoteID 到 SessionStore | SDK开发者 | V2.0 | 待实现 |
| Raw 解析失败 | 新接入不可用 | 报文不完整或字段异常 | 6 | 5 | 3 | 90 | Low | 返回结构化错误（字段路径 + 原因） | SDK开发者 | V2.0 | 待实现 |
| MultiRound 低置信度误判 | 错误映射被用于线上请求，导致会话串线或上下文错乱 | 多轮样本信息不足、字段同值冲突、推断规则命中歧义 | 8 | 5 | 5 | 200 | High | 强制输出字段级置信度；低于阈值进入 `pending_confirm`（仍创建 Session 并返回告警报告）；`failed` 返回错误且不创建 Session；一键回退 RawSpec 手工模式；上线前补充冲突样本回归集 | SDK开发者 | V2.7 | 待实现 |
| 鉴权冲突 | 401 / 403 | 同时出现多套认证信息 | 7 | 4 | 5 | 140 | Medium | 鉴权冲突直接失败，不自动猜测；日志脱敏处理 | SDK开发者 | V2.0 | 待实现 |

---

# 7. 部署与回滚策略

本模块为 Go SDK 库（非独立服务），无独立部署单元。

- **发布方式**：随 SDK 版本发布（Go module tag），接入方按需升级
- **回滚方式**：接入方回退 go.mod 中的 SDK 版本即可，无数据迁移
- **兼容性说明**：本版本为非兼容重构，接入方升级需按 [5. 关联分析] 中的影响点清单同步改造调用代码

---

# 8. 验证策略

## 8.1. 测试覆盖矩阵

| 需求 ID | 测试类型 | 测试场景 | 预期结果 |
|---------|---------|---------|---------|
| REQ-001 | 单元测试 | 创建 Session 不传 mode | 返回错误，拒绝执行 |
| REQ-001 | 单元测试 | 创建 Session 传 remote_session / local_history | 成功创建，Mode() 返回正确值 |
| REQ-002 | 单元测试 | remote_session 首轮响应含 RemoteID | LocalID 与 RemoteID 独立，RemoteID 从响应正确提取 |
| REQ-002 | 单元测试 | local_history 模式 | RemoteID 始终为空 |
| REQ-003 | 单元测试 | RawSpec URL 缺 scheme | 自动补 https://，编译成功 |
| REQ-003 | 单元测试 | RawSpec mode 缺失 | 返回字段级错误 |
| REQ-003 | 单元测试 | remote_session 模式无 session_id 占位符 | 返回占位符缺失错误 |
| REQ-004 | 单元测试 | 每轮对话后检查 SessionStore | Messages 包含完整 user + assistant 历史 |
| REQ-005 | 单元测试 | Headers 含 Bearer Token | 提取为 bearer_token 类型 Credential |
| REQ-005 | 单元测试 | Headers 同时含 Bearer + Basic | 返回冲突错误 |
| REQ-006 | 单元测试 | input 含双引号 `"` | 渲染后 JSON 合法，引号被转义为 `\"` |
| REQ-006 | 单元测试 | input 含换行符 `\n` | 渲染后 JSON 合法，换行被转义为 `\\n` |
| REQ-007 | 单元测试 | SSE 响应含 `data: ` 前缀 | 正确剥离前缀并解析 JSON |
| REQ-007 | 单元测试 | SSE 响应含 `data: [DONE]` | 流正常终止 |
| REQ-007 | 单元测试 | SSE 响应含空行和 `retry:` 行 | 跳过，不报错 |
| REQ-008 | 单元测试 | HTTP 响应 Content-Encoding: gzip | 自动解压，body 正常解析 |
| REQ-009 | 单元测试 | OnError=continue，单轮失败 | 记录错误，继续下一轮 |
| REQ-009 | 单元测试 | OnError=abort，单轮失败 | 立即终止，返回错误 |
| REQ-010 | 单元测试 | MaxMessages=3，历史有 10 条 | 仅携带最近 3 条历史 |
| REQ-010 | 单元测试 | MaxTokens=100，历史估算超限 | 裁剪至 Token 阈值以内 |
| REQ-011 | 单元测试 | ChainFields 列表非空，某条 ExtractOnEvent 为空 | 允许通过，空表示按任意事件帧提取 |
| REQ-011 | 单元测试 | ChainFields 列表非空，占位符格式不符合 $$$NAME$$$ | 初始化报错 |
| REQ-011 | 单元测试 | 首轮流式响应 message_end 含 message_id，第二轮请求体含 parent_message_id | parent_message_id 正确注入提取值 |
| REQ-011 | 单元测试 | 首轮响应中 ExtractOnEvent 不匹配（event 类型不同） | 该链路字段不提取，第二轮对应字段被移除 |
| REQ-011 | 单元测试 | 多个 ChainField 同时配置 | 各字段独立提取，互不干扰 |
| REQ-012 | 单元测试 | happy path：MultiRoundSpec 提供完整 2~5 轮请求与响应（默认建议3~5轮） | 成功生成 `GenericProfile + InferenceReport`，状态为 `auto_confirmed` 或 `pending_confirm` |
| REQ-012 | 单元测试 | edge：2~5 轮报文包含无关字段与字段顺序变化 | 推断结果不受顺序影响，无关字段不进入关键映射 |
| REQ-012 | 单元测试 | error：轮次数 <2 或 >5，或任一轮缺失 request/response | 返回 `"generic: multi_round_spec requires 2-5 rounds with request/response in each round"` 错误 |
| REQ-012 | 单元测试 | state：同一 MultiRoundSpec（含 2 轮输入场景）重复推断两次 | 输出字段分类与置信度稳定（可复现） |
| REQ-013 | 单元测试 | happy path：可同时识别 input/session_id/chain/dynamic/static | 五类字段均被正确分类并写入报告 |
| REQ-013 | 单元测试 | edge：同一路径存在多分类候选（同值冲突） | 报告标记冲突并进入确认流程 |
| REQ-013 | 单元测试 | error：请求或响应 Body 非法 JSON | 返回字段级解析错误，状态 `failed` |
| REQ-013 | 单元测试 | state：timestamp/nonce 在两轮变化 | 字段被归类为 `dynamic`，不被固化为 static |
| REQ-014 | 单元测试 | happy path：OverallConfidence 高于阈值且无冲突 | 报告状态 `auto_confirmed`，创建 Session |
| REQ-014 | 单元测试 | edge：OverallConfidence 恰好等于阈值 | 报告状态 `auto_confirmed`，创建 Session |
| REQ-014 | 单元测试 | error：OverallConfidence 低于阈值 | 报告状态 `pending_confirm`，仍创建 Session 且返回告警与 InferenceReport |
| REQ-014 | 单元测试 | state（Phase-2 验收：确认态持久化）：用户确认后再次加载同一推断结果 | 状态保持为已确认，不重复触发确认流程（Phase-1 不要求） |
| REQ-015 | 单元测试 | happy path：自动推断失败后切换 RawSpec 手工模式 | 手工模式可成功编译并创建 Session |
| REQ-015 | 单元测试 | edge：仅部分字段推断失败 | 允许保留可用结果并进入手工补全流程 |
| REQ-015 | 单元测试 | error：推断失败且未提供手工回退配置 | 返回明确错误并给出回退提示 |
| REQ-015 | 单元测试 | state：回退成功后继续多轮会话 | 会话状态可持续推进，无额外分支错误 |
| REQ-016 | 单元测试 | happy path：自动推断结果包含 FlowSpecMeta | `FlowSpecMeta.Version=v1alpha1` 且 `Source=MultiRoundSpec` |
| REQ-016 | 单元测试 | edge：存在未来扩展字段（unknown extension） | 保持透传，不影响当前解析 |
| REQ-016 | 单元测试 | error：FlowSpecMeta.Version 缺失 | 校验失败并返回结构化错误 |
| REQ-016 | 单元测试 | state：推断结果序列化/反序列化后再编译 | GenericProfile 与报告关键字段保持一致 |

## 8.2. 测试要求

- 所有需求 ID 均有对应测试用例，且测试通过后方可提交
- 流式解析相关测试需构造真实 SSE / NDJSON 格式的 mock 响应
- SessionStore 故障场景通过 mock 注入模拟

---

# 9. 变更控制

## 9.1. 变更列表

**最初版本的设计中本节内容应当为空。**

| 变更 ID | 变更章节 | 变更内容 | 变更原因 | 对已有功能/设计的影响 | 关联需求/代码 | 审批人 | 日期 |
|--------|---------|---------|---------|-------------------|-------------|-------|------|
| — | — | — | — | — | — | — | — |

---

# 10. 修订记录

| 修订版本号 | 作者 | 日期 | 简要说明 |
|-----------|------|------|---------|
| V2.0 | 徐文彬 | 2026-03-06 | 重构版：显式会话模式、双 ID 分离、非兼容改造 |
| V2.1 | 徐文彬 | 2026-03-06 | 补充：JSON 模板转义规范、SSE 解析规范、gzip 处理、OnError 策略、HistoryWindow、RemoteID 持久化时机、URL 合成规则 |
| V2.2 | 徐文彬 | 2026-03-06 | 代码扫描更新关联影响点清单：新增 SSE 解析、gzip 非流式/流式、JSON 模板渲染、HistoryWindow 落地、OnErrorStrategy、流式 RemoteID 持久化共 6 类新增条目 |
| V2.3 | 徐文彬 | 2026-03-06 | 修正会话模型设计：取消双 ID 分离，改为单一 SessionID；增加 DR-004 设计决策记录 |
| V2.4 | 徐文彬 | 2026-03-06 | 补充 isDifyProvider 特判影响点；补充 Test()/TestWith() mode 校验豁免策略（惰性校验） |
| V2.5 | 徐文彬 | 2026-03-06 | 补充流式/非流式响应影响分析：Chat() 需按 mode 控制注入与提取；ChatStream() 额外需在首个 chunk 提取到 SessionID 时立即持久化（不等流结束），修复流中断时 SessionID 丢失问题；3.4 新增流式与非流式行为差异对比表 |
| V2.6 | 徐文彬 | 2026-03-07 | 新增 REQ-011 ChainFields 多轮字段链路传递：新增 ChainField 结构体（Placeholder/ResponsePath/ExtractOnEvent 三字段，传了则全部必填）；GenericProfile.Stream 新增 ChainFields；RawIntegrationSpec 新增 ChainFields；ChatRequest/StreamChunk 新增 ChainValues；Session 内部新增 chainValues 状态；3.4/3.5/3.6 内部逻辑同步更新；补充 REQ-011 测试矩阵 5 条 |
| V2.7 | 徐文彬 | 2026-03-08 | 新增 MultiRoundSpec（2~5轮，默认建议3~5轮）自动推断设计：补充 REQ-012~REQ-016；3.6 增加多轮推断数据结构、字段分类规则（input/session_id/chain/dynamic/static）、冲突与置信度确认流程、失败回退手工模式、前向兼容 FlowSpec 的输出契约（GenericProfile + InferenceReport）；补充 FMEA 与测试矩阵 |
| V2.8 | 徐文彬 | 2026-03-08 | 文档同步当前实现：移除双轮独立方案描述与相关 API/示例；统一为仅保留 MultiRound 自动推断；接入口径改为“业务层直接以 2 轮数据构造 MultiRoundSpec”。 |
| V2.9 | 徐文彬 | 2026-03-09 | 全文统一三态会话口径：`auto_confirmed` 创建 Session；`pending_confirm` 也创建 Session（保留告警并返回 InferenceReport 供人工复核）；`failed` 返回 error 且不创建 Session；同步更新流程、异常、Phase、FMEA、测试矩阵。 |
