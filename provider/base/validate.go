package base

import (
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrUnsupportedImageFormat 不支持的图片格式
	ErrUnsupportedImageFormat = errors.New("client: unsupported image format")

	// ErrEmptyImageData 图片数据为空
	ErrEmptyImageData = errors.New("client: empty image data")

	// ErrUnsupportedPartType ContentPart.Type 未被识别（调用方可用 errors.Is 区分
	// "图片数据格式错误" 与 "整个 part 类型本身就不支持"，例如拼错 "image" 漏掉 "_url"）。
	ErrUnsupportedPartType = errors.New("client: unsupported part type")
)

// 支持的图片 MIME 类型白名单
var supportedImageMIMETypes = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
	"image/jpg":  true,
	"image/webp": true,
	"image/gif":  true,
}

// 已知 ContentPart.Type 白名单（含已实现与预留占位）。
// 预留类型（video_url/audio_url）当前 provider 层未实现，validate 仅放行，
// 实际处理由后续 provider 接入；非白名单值视为拼写错误（如 "image" 漏掉 "_url"），
// 由 ValidateContentParts 直接返回 ErrUnsupportedPartType 而非被静默丢弃。
var knownPartTypes = map[string]bool{
	"text":      true,
	"image_url": true,
	"video_url": true, // 预留，provider 未实现
	"audio_url": true, // 预留，provider 未实现
}

// ValidateContentParts 校验多模态内容块。
// 行为：
//   - 未知 Type（不在 knownPartTypes 白名单内）→ 返回 ErrUnsupportedPartType；
//   - "image_url" → 进行 Data/MIMEType 完整性与白名单校验；
//   - "text" / "video_url" / "audio_url" → 放行（后两者交给未来 provider 实现处理）。
func ValidateContentParts(parts []ContentPart) error {
	for i, part := range parts {
		if !knownPartTypes[part.Type] {
			return fmt.Errorf("%w: part[%d] type=%q (known: text, image_url; reserved: video_url, audio_url)",
				ErrUnsupportedPartType, i, part.Type)
		}
		if part.Type != "image_url" {
			continue
		}
		// image_url 深度校验
		if part.Data == "" {
			return fmt.Errorf("%w: part[%d] has empty data", ErrEmptyImageData, i)
		}
		if part.MIMEType == "" {
			return fmt.Errorf("%w: part[%d] has empty mime_type", ErrUnsupportedImageFormat, i)
		}
		mimeType := strings.ToLower(part.MIMEType)
		if !supportedImageMIMETypes[mimeType] {
			return fmt.Errorf("%w '%s', only PNG/JPEG/WEBP/GIF allowed",
				ErrUnsupportedImageFormat, part.MIMEType)
		}
	}
	return nil
}

// randomHex8 生成 8 位十六进制随机字符串
func randomHex8() string {
	b := make([]byte, 4)
	rand.Read(b)
	return fmt.Sprintf("%08x", b)
}

// InferFilenameFromMIME 根据 MIME 类型推断文件名（扩展名）。
//
// 注意：调用此函数前应先通过 ValidateContentParts() 验证格式。
// 本函数假设输入已通过验证（小写 + 白名单），但仍提供防御性错误处理。
//
// 参数:
//   - mimeType: 图片的 MIME 类型（如 "image/png", "image/jpeg"）
//
// 返回:
//   - filename: 推断的文件名（如 "image_a1b2c3d4.png"）
//   - error: 不支持的格式时返回 ErrUnsupportedImageFormat
func InferFilenameFromMIME(mimeType string) (string, error) {
	mimeType = strings.ToLower(mimeType)

	switch mimeType {
	case "image/png":
		return "image_" + randomHex8() + ".png", nil
	case "image/jpeg", "image/jpg":
		return "image_" + randomHex8() + ".jpg", nil
	case "image/webp":
		return "image_" + randomHex8() + ".webp", nil
	case "image/gif":
		return "image_" + randomHex8() + ".gif", nil
	default:
		return "", fmt.Errorf("%w '%s'", ErrUnsupportedImageFormat, mimeType)
	}
}
