# 配置生成器使用指南

## 概述

配置生成器（config-generator）是一个自动化工具，用于从 AI 模型的请求响应包文档生成标准化配置文件。它可以自动识别认证方式、提取关键参数，并生成 YAML 配置和 Go 代码两种格式的配置示例。

## 功能特性

- ✅ **自动解析请求响应包**：提取 URL、Headers、请求体、响应体
- ✅ **智能识别认证方式**：Bearer Token、API Key、Cookie、自定义 Header
- ✅ **自动提取额外字段**：识别非标准 OpenAI 字段（如 `group`）
- ✅ **双重输出格式**：同时生成 YAML 配置和 Go 内存构造代码
- ✅ **敏感信息保护**：自动替换 Token/Cookie 为占位符
- ✅ **批量生成支持**：一次处理多个平台配置
- ✅ **完整使用示例**：生成的文档包含可运行的示例代码

## 安装

### 方式 1: 直接运行（推荐开发环境）

```bash
go run ./tools/config-generator/cmd/config-generator [参数]
```

### 方式 2: 编译后使用（推荐生产环境）

```bash
# 编译
go build -o bin/config-generator ./tools/config-generator/cmd/config-generator

# 运行
./bin/config-generator [参数]
```

### 方式 3: 安装到系统

```bash
go install ./tools/config-generator/cmd/config-generator
config-generator [参数]
```

## 使用方法

### 命令行参数

```
-input string
    输入请求响应包 Markdown 路径（必需）

-output string
    输出 Markdown 路径（必需）

-platform string
    平台名称（可选，用于多平台输入场景）
```

### 基本用法

#### 1. 生成单个平台配置

```bash
# 生成 elysiver 平台配置
go run ./tools/config-generator/cmd/config-generator \
  -input docs/AI模型的请求响应包.md \
  -output generated/elysiver-config.md \
  -platform elysiver

# 生成 deepseek 平台配置
go run ./tools/config-generator/cmd/config-generator \
  -input docs/AI模型的请求响应包.md \
  -output generated/deepseek-config.md \
  -platform deepseek
```

#### 2. 批量生成所有平台配置

```bash
# 输出到目录，自动识别并生成所有平台
go run ./tools/config-generator/cmd/config-generator \
  -input docs/AI模型的请求响应包.md \
  -output generated/
```

生成的文件：
- `generated/elysiver-config.md`
- `generated/deepseek-config.md`
- `generated/openai-config.md`（如果有）

#### 3. 自动识别单个平台（不指定 -platform）

```bash
# 如果输入文件只包含一个平台，自动识别
go run ./tools/config-generator/cmd/config-generator \
  -input docs/single-platform.md \
  -output generated/config.md
```

## 输入文件格式

### 标准格式

工具需要解析以下格式的 Markdown 文档：

```markdown
## 1、平台名称（如 elysiver.h-e.top）
URL：https://example.com/api/path

​```yaml
POST /api/path HTTP/1.1
Host: example.com
Authorization: Bearer sk-xxx
Custom-Header: value
Content-Type: application/json

{"model":"xxx","messages":[...],"custom_field":"value"}
​```

​```yaml
HTTP/1.1 200 OK
Content-Type: application/json

{"id":"xxx","object":"chat.completion",...}
​```

## 2、另一个平台
...
```

### 关键要素

1. **平台标识**：以 `## 数字、平台名称` 开头
2. **URL 行**：`URL：https://...` 或 `POST /path HTTP/1.1`
3. **请求 Headers**：识别 `Authorization`、`Cookie`、自定义 Header
4. **请求体**：JSON 格式，自动提取额外字段
5. **响应体**：用于判断是否 OpenAI 兼容格式

## 输出文档格式

生成的 Markdown 文档包含以下部分：

### 1. 平台信息

```markdown
## 平台信息

- 名称: elysiver.h-e.top
- API 地址: https://elysiver.h-e.top
- 接口路径: /pg/chat/completions
- 认证方式: 自定义 Header / Cookie
```

### 2. YAML 配置

```yaml
auth:
  store:
    type: file
    path: "./credentials.json"

providers:
  - name: "elysiver"
    type: "openai_compat"
    base_url: "https://elysiver.h-e.top"
    path: "/pg/chat/completions"
    auth_ref: "elysiver_cred"
    extra_body:
      group: "default"

credentials:
  - id: "elysiver_cred"
    auth_type: "none"
    headers:
      Cookie: "REPLACE_WITH_COOKIE"
      New-Api-User: "REPLACE_WITH_USER_ID"
```

### 3. Go 内存构造代码

```go
func NewElysiverConfig() *config.Config {
    return &config.Config{
        Providers: []config.ProviderConfig{
            {
                Name:    "elysiver",
                Type:    "openai_compat",
                BaseURL: "https://elysiver.h-e.top",
                Path:    "/pg/chat/completions",
                AuthRef: "elysiver_cred",
                ExtraBody: map[string]any{
                    "group": "default",
                },
            },
        },
        Credentials: []*auth.Credential{
            {
                ID:       "elysiver_cred",
                AuthType: auth.AuthTypeNone,
                Headers: map[string]string{
                    "Cookie":       "REPLACE_WITH_COOKIE",
                    "New-Api-User": "REPLACE_WITH_USER_ID",
                },
            },
        },
    }
}
```

### 4. 完整使用示例

包含初始化客户端和发送请求的完整代码。

### 5. 字段说明

详细解释每个配置字段的含义和使用场景。

### 6. 注意事项

特定平台的注意事项（如 Cookie 过期、自定义参数等）。

## 高级用法

### API 方式调用

除了 CLI 工具，也可以在代码中直接调用 API：

```go
package main

import (
    "log"
    "github.com/Michaelxwb/ai-api-sdk/tools/config-generator"
)

func main() {
    // 生成单个平台配置
    err := generator.GenerateConfigForPlatform(
        "docs/AI模型的请求响应包.md",
        "generated/elysiver-config.md",
        "elysiver",
    )
    if err != nil {
        log.Fatal(err)
    }

    // 批量生成所有平台
    err = generator.GenerateConfig(
        "docs/AI模型的请求响应包.md",
        "generated/",
    )
    if err != nil {
        log.Fatal(err)
    }
}
```

### 自定义模板

如果需要自定义输出格式，可以修改 `tools/config-generator/templates.go`：

```go
// 修改 YAML 模板
const yamlTemplate = `...`

// 修改 Go 代码模板
const goCodeTemplate = `...`

// 修改输出文档模板
const markdownTemplate = `...`
```

### 扩展解析规则

如需支持更多认证方式或字段识别，修改 `tools/config-generator/parser.go`：

```go
// 添加新的认证类型识别
func detectAuthType(headers map[string]string) AuthInfo {
    // 添加自定义逻辑
    if customAuth := headers["X-Custom-Auth"]; customAuth != "" {
        return AuthInfo{
            Type: "custom",
            CustomHeaders: map[string]string{
                "X-Custom-Auth": "...",
            },
        }
    }
    // ...
}

// 添加新的额外字段识别
func detectExtraBody(requestBody map[string]any) map[string]any {
    // 添加自定义字段白名单
    standardFields := []string{
        "model", "messages", "temperature", "max_tokens",
        "stream", "top_p", "frequency_penalty", "presence_penalty",
        // 添加更多标准字段
    }
    // ...
}
```

## 常见使用场景

### 场景 1: 接入新的 AI 平台

```bash
# 1. 从浏览器开发者工具复制请求响应包到 docs/new-platform.md
# 2. 生成配置
go run ./tools/config-generator/cmd/config-generator \
  -input docs/new-platform.md \
  -output generated/new-platform-config.md

# 3. 复制生成的 YAML 配置到 config.yaml
# 4. 替换占位符（Token、Cookie 等）
# 5. 测试连通性
go run ./examples/04-connectivity-test/main.go
```

### 场景 2: 批量迁移多个平台

```bash
# 1. 准备包含所有平台的请求响应包文档
# 2. 批量生成配置
go run ./tools/config-generator/cmd/config-generator \
  -input docs/all-platforms.md \
  -output generated/

# 3. 逐个平台验证配置
for file in generated/*.md; do
  echo "验证 $file"
  # 复制配置并测试
done
```

### 场景 3: 集成到 CI/CD 流程

```yaml
# .github/workflows/generate-configs.yml
name: Generate Platform Configs

on:
  push:
    paths:
      - 'docs/AI模型的请求响应包.md'

jobs:
  generate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2

      - name: Setup Go
        uses: actions/setup-go@v2
        with:
          go-version: '1.23'

      - name: Generate configs
        run: |
          go run ./tools/config-generator/cmd/config-generator \
            -input docs/AI模型的请求响应包.md \
            -output generated/

      - name: Commit generated configs
        run: |
          git add generated/
          git commit -m "Auto-generate platform configs"
          git push
```

### 场景 4: 动态生成用户自定义配置

```go
// 用户上传请求响应包 → 生成配置 → 保存到数据库
func HandleUserPlatform(requestPackage string) error {
    // 1. 保存到临时文件
    tmpFile := "/tmp/user-platform.md"
    os.WriteFile(tmpFile, []byte(requestPackage), 0644)

    // 2. 生成配置
    outputFile := "/tmp/user-platform-config.md"
    err := generator.GenerateConfig(tmpFile, outputFile)
    if err != nil {
        return err
    }

    // 3. 解析生成的配置
    config := parseGeneratedConfig(outputFile)

    // 4. 保存到数据库
    db.SavePlatformConfig(config)

    return nil
}
```

## 测试

运行单元测试：

```bash
# 运行所有测试
go test ./tools/config-generator/...

# 运行单个测试
go test ./tools/config-generator/ -run TestGenerateConfig

# 查看测试覆盖率
go test ./tools/config-generator/... -cover
```

## 故障排查

### 问题 1: 无法识别平台

**症状**：提示 "未解析到平台配置"

**解决**：
1. 检查输入文件格式是否符合要求
2. 确认平台标识格式：`## 数字、平台名称`
3. 确认 URL 行格式：`URL：https://...` 或 `POST /path HTTP/1.1`

### 问题 2: 认证方式识别错误

**症状**：生成的认证类型不正确

**解决**：
1. 检查 Headers 中的 `Authorization` 字段
2. 对于自定义 Header 认证，确保 Headers 清晰列出
3. 必要时手动修改生成的配置

### 问题 3: 额外字段未识别

**症状**：`extra_body` 字段为空

**解决**：
1. 检查请求体 JSON 格式是否正确
2. 确认字段确实是非标准字段（不在 OpenAI 标准字段列表中）
3. 必要时在 `parser.go` 中添加字段白名单

### 问题 4: 生成的代码无法编译

**症状**：复制代码后编译错误

**解决**：
1. 检查 import 路径是否正确
2. 确认 SDK 版本兼容
3. 替换占位符为实际值
4. 运行 `go mod tidy`

## 最佳实践

1. **敏感信息保护**
   - 生成的配置自动替换敏感信息为占位符
   - 替换实际值后不要提交到版本控制
   - 使用环境变量或配置中心管理敏感信息

2. **配置验证**
   - 生成配置后先用连通性测试验证
   - 使用 `examples/09-connectivity-test` 测试配置

3. **文档维护**
   - 保存原始请求响应包到 `docs/` 目录
   - 生成的配置保存到 `generated/` 目录
   - 在 `.gitignore` 中排除敏感配置文件

4. **版本管理**
   - 为不同版本的 API 生成独立配置
   - 在文件名中包含版本号：`platform-v2-config.md`

5. **团队协作**
   - 将标准配置模板提交到版本控制
   - 使用占位符代替实际凭证
   - 在 README 中说明如何获取实际凭证

## 相关文档

- [自定义网关配置指南](../../docs/custom-gateway-guide.md) - 详细配置说明
- [配置指南](../../docs/configuration.md) - 通用配置说明
- [快速开始](../../docs/quickstart.md) - SDK 使用入门
- [示例代码](../../examples/README.md) - 完整示例

## 技术支持

如遇到问题，请：
1. 查看 [常见问题](#故障排查)
2. 查看 [GitHub Issues](https://github.com/Michaelxwb/ai-api-sdk/issues)
3. 提交新的 Issue 并附上输入文件和错误信息
