# AI API SDK 文档

## 快速导航

### 入门指南
- [快速开始](quickstart.md) - 5 分钟上手
- [配置指南](configuration.md) - 配置文件详解
- [示例代码](examples.md) - 完整示例矩阵

### 使用指南
- [基础使用](usage-guide.md) - 单轮对话、流式对话、**连通性测试**
- [多轮对话](session-tutorial.md) - 会话管理和存储
- [API 参考](api-reference.md) - 完整 API 文档

### 架构设计
- [架构概览](architecture.md) - 整体架构和模块划分（已合并数据流和流式解析）
- ~~[数据流详解](data-flow.md)~~ - *已合并到架构概览*

### 高级主题
- [数据库存储](session-database.md) - PostgreSQL/MySQL/SQLite
- [Session API](session-api.md) - Session 接口详细说明
- [流式迁移指南](migration-to-streaming.md) - 升级到流式优先架构

### 扩展开发
- [自定义网关配置指南](custom-gateway-guide.md) - 非标准模型接入、认证方式、数据库表结构
- [配置生成器使用指南](config-generator-usage.md) - 自动生成平台配置的工具

### 开发者文档
- [Session 实现细节](internal/session-implementation.md) - 内部实现（开发者）

## 按场景查找

### 我想...
- **快速接入一个 AI 模型** → [快速开始](quickstart.md)
- **配置多个 Provider** → [配置指南](configuration.md)
- **接入自定义网关/非标准 AI 模型** → [自定义网关配置指南](custom-gateway-guide.md)
- **自动生成平台配置** → [配置生成器使用指南](config-generator-usage.md)
- **实现多轮对话** → [Session 教程](session-tutorial.md)
- **使用数据库存储会话** → [数据库存储](session-database.md)
- **实现流式输出（打字机效果）** → [使用指南 - 流式对话](usage-guide.md#流式对话)
- **测试 Provider 配置是否正确** → [使用指南 - 连通性测试](usage-guide.md#连通性测试)
- **理解项目架构** → [架构概览](architecture.md)
- **查看完整示例** → [示例代码](examples.md)
- **查看 Session API** → [Session API](session-api.md)

## 文档精简说明

本文档系统已于 2026-02 进行精简重构：
- 合并重复内容（architecture.md ← data-flow.md + streaming-providers.md）
- 精简过长文档（session-tutorial.md -75%、session-api.md -56%、session-database.md -45%）
- 明确职责边界（入门/使用/架构/高级）
