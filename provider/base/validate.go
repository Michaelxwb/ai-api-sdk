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
)

// 支持的图片 MIME 类型白名单
var supportedImageMIMETypes = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
	"image/jpg":  true,
	"image/webp": true,
	"image/gif":  true,
}

// ValidateContentParts 校验多模态内容块。
// 对 Parts 中的图片进行格式和数据完整性校验。
func ValidateContentParts(parts []ContentPart) error {
	for i, part := range parts {
		if part.Type == "image_url" {
			// 校验 Data 非空
			if part.Data == "" {
				return fmt.Errorf("%w: part[%d] has empty data", ErrEmptyImageData, i)
			}

			// 校验 MIMEType 非空
			if part.MIMEType == "" {
				return fmt.Errorf("%w: part[%d] has empty mime_type", ErrUnsupportedImageFormat, i)
			}

			// 校验 MIMEType 在白名单内（大小写不敏感）
			mimeType := strings.ToLower(part.MIMEType)
			if !supportedImageMIMETypes[mimeType] {
				return fmt.Errorf("%w '%s', only PNG/JPEG/WEBP/GIF allowed",
					ErrUnsupportedImageFormat, part.MIMEType)
			}
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
