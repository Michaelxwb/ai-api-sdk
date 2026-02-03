package generator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"text/template"
)

type TemplateData struct {
	PlatformName      string
	ProviderName      string
	BaseURL           string
	Path              string
	AuthType          string
	AuthTypeDesc      string
	BearerToken       string
	APIKey            string
	ExtraBody         map[string]any
	ExtraBodyKeys     []string
	ExtraBodyYAML     map[string]string
	CustomHeaders     map[string]string
	CustomHeaderKeys  []string
	QueryParams       map[string]string
	QueryParamKeys    []string
	FuncName          string
	DefaultModel      string
	YAMLConfig        string
	GoCode            string
	FieldDescriptions string
	Notes             string
}

func BuildTemplateData(p PlatformInfo, authInfo AuthInfo) TemplateData {
	providerName := p.ProviderName
	if providerName == "" {
		providerName = "provider"
	}
	funcName := toCamel(providerName)
	if p.Name == "" {
		p.Name = providerName
	}
	if p.Path == "" {
		p.Path = "/chat/completions"
	}
	if p.BaseURL == "" {
		p.BaseURL = "https://example.com"
	}
	if p.DefaultModel == "" {
		p.DefaultModel = "YOUR_MODEL"
	}
	customHeaders := authInfo.CustomHeaders
	customHeaderKeys := sortedKeys(customHeaders)
	queryKeys := sortedKeys(p.QueryParams)
	extraBody := p.ExtraBody
	extraKeys := sortedAnyKeys(extraBody)
	extraYAML := map[string]string{}
	for k, v := range extraBody {
		extraYAML[k] = formatYAMLValue(v)
	}
	return TemplateData{
		PlatformName:      p.Name,
		ProviderName:      providerName,
		BaseURL:           p.BaseURL,
		Path:              p.Path,
		AuthType:          authInfo.Type,
		AuthTypeDesc:      authTypeDesc(authInfo),
		BearerToken:       authInfo.BearerToken,
		APIKey:            authInfo.APIKey,
		ExtraBody:         extraBody,
		ExtraBodyKeys:     extraKeys,
		ExtraBodyYAML:     extraYAML,
		CustomHeaders:     customHeaders,
		CustomHeaderKeys:  customHeaderKeys,
		QueryParams:       p.QueryParams,
		QueryParamKeys:    queryKeys,
		FuncName:          funcName,
		DefaultModel:      p.DefaultModel,
		FieldDescriptions: defaultFieldDescriptions(),
		Notes:             defaultNotes(p, authInfo),
	}
}

func RenderYAML(data TemplateData) (string, error) {
	tpl, err := template.New("yaml").Funcs(template.FuncMap{
		"yamlValue": func(key string) string { return data.ExtraBodyYAML[key] },
	}).Parse(yamlTemplate)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()) + "\n", nil
}

func RenderGo(data TemplateData) (string, error) {
	tpl, err := template.New("go").Funcs(template.FuncMap{
		"formatValue": formatGoValue,
		"authConst":   authTypeConst,
	}).Parse(goTemplate)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()) + "\n", nil
}

func RenderMarkdown(data TemplateData) (string, error) {
	tpl, err := template.New("md").Parse(markdownTemplate)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()) + "\n", nil
}

func formatYAMLValue(v any) string {
	switch val := v.(type) {
	case string:
		return strconv.Quote(val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	case float64:
		if float64(int64(val)) == val {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%g", val)
	case int, int32, int64:
		return fmt.Sprintf("%d", val)
	case uint, uint32, uint64:
		return fmt.Sprintf("%d", val)
	default:
		raw, err := json.Marshal(val)
		if err != nil {
			return strconv.Quote("UNSUPPORTED_VALUE")
		}
		return strconv.Quote(string(raw))
	}
}

func formatGoValue(v any) string {
	switch val := v.(type) {
	case string:
		return strconv.Quote(val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	case float64:
		if float64(int64(val)) == val {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%g", val)
	case int, int32, int64, uint, uint32, uint64:
		return fmt.Sprintf("%v", val)
	default:
		return fmt.Sprintf("%#v", val)
	}
}

func authTypeConst(authType string) string {
	switch authType {
	case "bearer_token":
		return "auth.AuthTypeBearerToken"
	case "api_key":
		return "auth.AuthTypeAPIKey"
	case "none":
		return "auth.AuthTypeNone"
	default:
		return "auth.AuthTypeNone"
	}
}

func authTypeDesc(authInfo AuthInfo) string {
	switch authInfo.Type {
	case "bearer_token":
		return "Bearer Token"
	case "api_key":
		return "API Key（Header）"
	case "none":
		if len(authInfo.CustomHeaders) > 0 {
			return "自定义 Header / Cookie"
		}
		return "无认证"
	default:
		return "未知"
	}
}

func toCamel(name string) string {
	if name == "" {
		return "Custom"
	}
	parts := strings.FieldsFunc(name, func(r rune) bool {
		return r == '_' || r == '-' || r == '.' || r == ' '
	})
	for i := range parts {
		if parts[i] == "" {
			continue
		}
		parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
	}
	out := strings.Join(parts, "")
	if out == "" {
		return "Custom"
	}
	return out
}

func defaultFieldDescriptions() string {
	return strings.TrimSpace(`- base_url: API 网关的根地址，通常是协议 + 域名，例如 https://example.com。
- path: 请求路径，默认是 /chat/completions；当网关路径不一致时需要覆盖。
- auth_type: 认证类型，支持 none / bearer_token / api_key；自定义 Header 使用 none + headers。
- headers: 自定义请求头（JSON），用于 Cookie、用户标识或网关签名等非标准认证信息。
- extra_body: 额外请求字段（JSON），会与标准请求体合并，例如 group / workspace / project。
- query_params: 追加到 URL 的 Query 参数（JSON），常用于租户或版本号。`)
}

func defaultNotes(p PlatformInfo, authInfo AuthInfo) string {
	var notes []string
	if p.IsStream {
		notes = append(notes, "该平台返回流式响应（text/event-stream），如需非流式请在请求体中显式设置 stream=false。")
	}
	if authInfo.Type == "none" && len(authInfo.CustomHeaders) > 0 {
		notes = append(notes, "使用自定义 Header 认证时请将 Cookie/用户标识放在 credential.headers 中，以便 SDK 自动注入。")
	}
	if authInfo.Type == "api_key" {
		notes = append(notes, "API Key 方式可通过 credential.metadata 配置 header_name / header_prefix 自定义 Header 名称与前缀。")
	}
	if len(notes) == 0 {
		notes = append(notes, "请确保在生产环境启用加密存储并定期轮换凭证。")
	}
	sort.Strings(notes)
	return strings.TrimSpace("- " + strings.Join(notes, "\n- "))
}

const yamlTemplate = `# {{ .PlatformName }} 平台配置（自定义网关示例）
# 说明：以下内容可直接保存为 config.yaml 使用

auth:
  store:
    type: file          # 凭证存储方式：file / db / memory
    path: "./credentials.json"
    encrypted: false

providers:
  - name: "{{ .ProviderName }}"           # provider 名称（后续调用使用）
    type: "openai_compat"       # OpenAI 兼容协议
    base_url: "{{ .BaseURL }}"   # API 根地址
    path: "{{ .Path }}"          # 接口路径
    auth_ref: "{{ .ProviderName }}_cred"  # 关联凭证 ID
{{- if .ExtraBodyKeys }}
    extra_body:
{{- range .ExtraBodyKeys }}
      {{ . }}: {{ yamlValue . }}
{{- end }}
{{- end }}

credentials:
  - id: "{{ .ProviderName }}_cred"
    provider: "openai_compat"
    auth_type: "{{ .AuthType }}"
{{- if eq .AuthType "bearer_token" }}
    access_token: "{{ .BearerToken }}"
{{- else if eq .AuthType "api_key" }}
    api_key: "{{ .APIKey }}"
{{- end }}
{{- if .CustomHeaderKeys }}
    headers:
{{- range .CustomHeaderKeys }}
      {{ . }}: "{{ index $.CustomHeaders . }}"
{{- end }}
{{- end }}
{{- if .QueryParamKeys }}
    query_params:
{{- range .QueryParamKeys }}
      {{ . }}: "{{ index $.QueryParams . }}"
{{- end }}
{{- end }}
`

const goTemplate = `package main

import (
    "github.com/Michaelxwb/ai-api-sdk/auth"
    "github.com/Michaelxwb/ai-api-sdk/config"
)

// {{ .PlatformName }} 平台配置（内存构造方式）
// 注意：示例中的 Token / Cookie 为占位符，请替换为真实值。
func New{{ .FuncName }}Config() *config.Config {
    return &config.Config{
        Auth: config.AuthConfig{
            Store: config.StoreConfig{
                Type:      "file",                // 凭证存储方式
                Path:      "./credentials.json",  // 凭证存储路径
                Encrypted: false,                  // 是否加密
            },
        },
        Providers: []config.ProviderConfig{
            {
                Name:    "{{ .ProviderName }}",    // provider 名称
                Type:    "openai_compat",          // OpenAI 兼容协议
                BaseURL: "{{ .BaseURL }}",          // API 根地址
                Path:    "{{ .Path }}",             // 接口路径
                AuthRef: "{{ .ProviderName }}_cred", // 绑定凭证
{{- if .ExtraBodyKeys }}
                ExtraBody: map[string]any{
{{- range .ExtraBodyKeys }}
                    "{{ . }}": {{ formatValue (index $.ExtraBody .) }},
{{- end }}
                },
{{- end }}
            },
        },
        Credentials: []*auth.Credential{
            {
                ID:       "{{ .ProviderName }}_cred", // 凭证 ID
                Provider: "openai_compat",           // Provider 类型
                AuthType: {{ authConst .AuthType }},
{{- if eq .AuthType "bearer_token" }}
                AccessToken: "{{ .BearerToken }}",
{{- else if eq .AuthType "api_key" }}
                APIKey: "{{ .APIKey }}",
{{- end }}
{{- if .CustomHeaderKeys }}
                Headers: map[string]string{
{{- range .CustomHeaderKeys }}
                    "{{ . }}": "{{ index $.CustomHeaders . }}",
{{- end }}
                },
{{- end }}
{{- if .QueryParamKeys }}
                QueryParams: map[string]string{
{{- range .QueryParamKeys }}
                    "{{ . }}": "{{ index $.QueryParams . }}",
{{- end }}
                },
{{- end }}
            },
        },
    }
}
`

const markdownTemplate = "# {{ .PlatformName }} 平台配置文档\n\n" +
	"## 平台信息\n\n" +
	"- 名称: {{ .PlatformName }}\n" +
	"- API 地址: {{ .BaseURL }}\n" +
	"- 接口路径: {{ .Path }}\n" +
	"- 认证方式: {{ .AuthTypeDesc }}\n\n" +
	"## 方式 1: YAML 配置\n\n" +
	"```yaml\n" +
	"{{ .YAMLConfig }}\n" +
	"```\n\n" +
	"## 方式 2: 内存构造（Go 代码）\n\n" +
	"```go\n" +
	"{{ .GoCode }}\n" +
	"```\n\n" +
	"## 使用示例\n\n" +
	"```go\n" +
	"package main\n\n" +
	"import (\n" +
	"    \"context\"\n\n" +
	"    \"github.com/Michaelxwb/ai-api-sdk/auth\"\n" +
	"    \"github.com/Michaelxwb/ai-api-sdk/client\"\n" +
	"    \"github.com/Michaelxwb/ai-api-sdk/config\"\n" +
	"    \"github.com/Michaelxwb/ai-api-sdk/provider/base\"\n" +
	")\n\n" +
	"func main() {\n" +
	"    // 方式一：从 YAML 加载配置\n" +
	"    cfg, _ := config.LoadConfig(\"config.yaml\")\n\n" +
	"    // 方式二：从内存构造配置\n" +
	"    cfg = New{{ .FuncName }}Config()\n\n" +
	"    // 初始化凭证管理器\n" +
	"    authStore := auth.NewFileStore(cfg.Auth.Store.Path)\n" +
	"    mgr, _ := auth.NewManager(authStore, &auth.RoundRobinSelector{})\n" +
	"    for _, cred := range cfg.Credentials {\n" +
	"        mgr.Register(cred)\n" +
	"    }\n\n" +
	"    // 创建客户端\n" +
	"    cli := client.NewClient(cfg, mgr)\n\n" +
	"    // 发起请求\n" +
	"    resp, _ := cli.NewSession(\"{{ .ProviderName }}\").Chat(context.Background(), base.ChatRequest{\n" +
	"        Model: \"{{ .DefaultModel }}\",\n" +
	"        Messages: []base.Message{\n" +
	"            {Role: \"user\", Content: \"测试\"},\n" +
	"        },\n" +
	"        Stream: false,\n" +
	"    })\n\n" +
	"    _ = resp\n" +
	"}\n" +
	"```\n\n" +
	"## 字段说明\n\n" +
	"{{ .FieldDescriptions }}\n\n" +
	"## 注意事项\n\n" +
	"{{ .Notes }}\n"
