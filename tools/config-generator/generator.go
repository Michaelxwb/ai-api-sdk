package generator

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type AuthInfo struct {
	Type          string
	BearerToken   string
	APIKey        string
	CustomHeaders map[string]string
}

// GenerateConfig 从 AI 模型请求响应包文件生成配置文档。
func GenerateConfig(inputPath, outputPath string) error {
	return generateConfig(inputPath, outputPath, "")
}

// GenerateConfigForPlatform 支持显式指定平台名称（用于多平台输入场景）。
func GenerateConfigForPlatform(inputPath, outputPath, platform string) error {
	return generateConfig(inputPath, outputPath, platform)
}

func generateConfig(inputPath, outputPath, platform string) error {
	if strings.TrimSpace(inputPath) == "" {
		return errors.New("inputPath 不能为空")
	}
	if strings.TrimSpace(outputPath) == "" {
		return errors.New("outputPath 不能为空")
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("读取输入文件失败: %w", err)
	}
	platforms, err := ParsePlatforms(string(data))
	if err != nil {
		return err
	}
	if len(platforms) == 0 {
		return errors.New("未解析到平台配置")
	}

	// 如果输出路径是目录，则生成全部平台
	if looksLikeDir(outputPath) {
		if err := os.MkdirAll(outputPath, 0o755); err != nil {
			return fmt.Errorf("创建输出目录失败: %w", err)
		}
		for i := range platforms {
			p := platforms[i]
			outPath := filepath.Join(outputPath, p.ProviderName+"-config.md")
			if err := generateOne(p, outPath); err != nil {
				return err
			}
		}
		return nil
	}

	target, err := selectPlatform(platforms, outputPath, platform)
	if err != nil {
		return err
	}
	return generateOne(*target, outputPath)
}

func generateOne(p PlatformInfo, outputPath string) error {
	authInfo := detectAuthType(p.Headers)
	if p.ExtraBody == nil {
		p.ExtraBody = detectExtraBody(p.RequestBody)
	}

	data := BuildTemplateData(p, authInfo)
	yamlConfig, err := RenderYAML(data)
	if err != nil {
		return err
	}
	data.YAMLConfig = yamlConfig
	goCode, err := RenderGo(data)
	if err != nil {
		return err
	}
	data.GoCode = goCode
	doc, err := RenderMarkdown(data)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("创建输出目录失败: %w", err)
	}
	if err := os.WriteFile(outputPath, []byte(doc), 0o644); err != nil {
		return fmt.Errorf("写入输出文件失败: %w", err)
	}
	return nil
}

func looksLikeDir(outputPath string) bool {
	if strings.HasSuffix(outputPath, string(os.PathSeparator)) {
		return true
	}
	if filepath.Ext(outputPath) == "" {
		return true
	}
	if info, err := os.Stat(outputPath); err == nil && info.IsDir() {
		return true
	}
	return false
}

func selectPlatform(platforms []PlatformInfo, outputPath, platform string) (*PlatformInfo, error) {
	if platform != "" {
		platform = strings.ToLower(platform)
		for i := range platforms {
			if matchPlatform(platforms[i], platform) {
				return &platforms[i], nil
			}
		}
		return nil, fmt.Errorf("未找到平台 %s，可选平台: %s", platform, listPlatformNames(platforms))
	}
	if len(platforms) == 1 {
		return &platforms[0], nil
	}
	base := strings.ToLower(filepath.Base(outputPath))
	for i := range platforms {
		if matchPlatform(platforms[i], base) {
			return &platforms[i], nil
		}
	}
	return nil, fmt.Errorf("输入文件包含多个平台，请在输出文件名中包含平台名或使用 --platform 指定，可选平台: %s", listPlatformNames(platforms))
}

func matchPlatform(p PlatformInfo, target string) bool {
	if target == "" {
		return false
	}
	name := strings.ToLower(p.Name)
	provider := strings.ToLower(p.ProviderName)
	return strings.Contains(name, target) || strings.Contains(provider, target) || strings.Contains(target, name) || strings.Contains(target, provider)
}

func listPlatformNames(platforms []PlatformInfo) string {
	items := make([]string, 0, len(platforms))
	for _, p := range platforms {
		name := p.Name
		if name == "" {
			name = p.ProviderName
		}
		items = append(items, name)
	}
	return strings.Join(items, ", ")
}

func detectAuthType(headers map[string]string) AuthInfo {
	lower := map[string]string{}
	for k, v := range headers {
		lower[strings.ToLower(k)] = v
	}

	if auth := lower["authorization"]; auth != "" {
		if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
			return AuthInfo{Type: "bearer_token", BearerToken: "REPLACE_WITH_YOUR_TOKEN"}
		}
		return AuthInfo{Type: "api_key", APIKey: "REPLACE_WITH_YOUR_API_KEY"}
	}
	if apiKey := firstNonEmpty(lower["x-api-key"], lower["api-key"], lower["apikey"], lower["x_api_key"]); apiKey != "" {
		return AuthInfo{Type: "api_key", APIKey: "REPLACE_WITH_YOUR_API_KEY"}
	}

	customHeaders := map[string]string{}
	for k, v := range lower {
		if isIgnoredHeader(k) {
			continue
		}
		if k == "authorization" {
			continue
		}
		if looksAuthHeader(k) {
			customHeaders[canonicalHeader(k)] = placeholderForHeader(k, v)
		}
	}
	if len(customHeaders) > 0 {
		return AuthInfo{Type: "none", CustomHeaders: customHeaders}
	}
	return AuthInfo{Type: "none"}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func isIgnoredHeader(keyLower string) bool {
	if keyLower == "" {
		return true
	}
	ignored := []string{
		"host", "connection", "content-length", "content-type", "accept", "accept-encoding", "accept-language",
		"origin", "referer", "user-agent", "cache-control", "pragma", "upgrade-insecure-requests",
		"sec-fetch-site", "sec-fetch-mode", "sec-fetch-dest", "sec-fetch-user",
		"sec-ch-ua", "sec-ch-ua-platform", "sec-ch-ua-mobile", "sec-ch-ua-model", "sec-ch-ua-arch", "sec-ch-ua-bitness",
		"sec-ch-ua-full-version", "sec-ch-ua-full-version-list", "sec-ch-ua-platform-version",
	}
	for _, item := range ignored {
		if keyLower == item {
			return true
		}
	}
	if strings.HasPrefix(keyLower, "sec-") {
		return true
	}
	return false
}

func looksAuthHeader(keyLower string) bool {
	if keyLower == "cookie" {
		return true
	}
	if strings.Contains(keyLower, "token") || strings.Contains(keyLower, "auth") || strings.Contains(keyLower, "key") || strings.Contains(keyLower, "signature") || strings.Contains(keyLower, "user") {
		return true
	}
	return false
}

func canonicalHeader(keyLower string) string {
	parts := strings.Split(keyLower, "-")
	for i := range parts {
		if len(parts[i]) == 0 {
			continue
		}
		parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
	}
	return strings.Join(parts, "-")
}

func placeholderForHeader(keyLower, value string) string {
	_ = value
	if keyLower == "cookie" {
		return "REPLACE_WITH_COOKIE"
	}
	if strings.Contains(keyLower, "token") {
		return "REPLACE_WITH_TOKEN"
	}
	if strings.Contains(keyLower, "key") {
		return "REPLACE_WITH_API_KEY"
	}
	if strings.Contains(keyLower, "user") {
		return "REPLACE_WITH_USER_ID"
	}
	return "REPLACE_WITH_HEADER_VALUE"
}
