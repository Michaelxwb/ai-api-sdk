# AI API SDK 文档

## 快速导航

### 入门指南
- [快速开始](quickstart.md) - 5 分钟上手
- [配置指南](configuration.md) - 配置文件详解
- [API 使用指南](api-guide.md) - 核心 API、连通性测试与最佳实践
- [示例代码](../examples/README.md) - 示例目录与运行方式

### Session 指南
- [Session 完整指南](session-guide.md) - Session 对象、SessionStore、数据库与最佳实践

### 架构设计
- [架构概览](architecture.md) - 整体架构和模块划分
- [Session 架构设计](internal/design-session-unified-architecture.md) - Session 统一架构设计
- [浏览器插件接入设计](internal/design-browser-plugin-integration.md) - 插件接入模块设计说明

### 扩展开发
- [自定义网关配置指南](custom-gateway-guide.md) - 非标准模型接入、认证方式、数据库表结构
- [自定义认证与签名指南](custom-auth-guide.md) - 非标准认证与签名扩展

## 按场景查找

### 我想...
- **快速接入一个 AI 模型** → [快速开始](quickstart.md)
- **配置多个 Provider** → [配置指南](configuration.md)
- **接入自定义网关/非标准 AI 模型** → [自定义网关配置指南](custom-gateway-guide.md)
- **实现多轮对话** → [Session 完整指南](session-guide.md)
- **浏览器插件接入** → [示例代码](../examples/README.md)（`examples/05-browser-plugin`）
- **查看插件接入设计** → [浏览器插件接入设计](internal/design-browser-plugin-integration.md)
- **使用数据库存储会话** → [Session 完整指南](session-guide.md)
- **实现流式输出（打字机效果）** → [API 使用指南](api-guide.md)
- **测试 Provider 配置是否正确** → [API 使用指南](api-guide.md)
- **理解项目架构** → [架构概览](architecture.md)
- **查看 Session 架构设计** → [Session 架构设计](internal/design-session-unified-architecture.md)
- **查看完整示例** → [示例代码](../examples/README.md)
