package base

import (
	"errors"
	"regexp"
	"testing"
)

func TestValidateContentParts(t *testing.T) {
	tests := []struct {
		name    string
		parts   []ContentPart
		wantErr error
	}{
		{
			name: "有效格式 - PNG",
			parts: []ContentPart{
				{Type: "text", Text: "描述图片"},
				{Type: "image_url", Data: "iVBORw0KGgo...", MIMEType: "image/png"},
			},
			wantErr: nil,
		},
		{
			name: "有效格式 - JPEG",
			parts: []ContentPart{
				{Type: "image_url", Data: "/9j/4AAQSkZJRg...", MIMEType: "image/jpeg"},
			},
			wantErr: nil,
		},
		{
			name: "有效格式 - WEBP",
			parts: []ContentPart{
				{Type: "image_url", Data: "UklGRiQAAABXRUJQ...", MIMEType: "image/webp"},
			},
			wantErr: nil,
		},
		{
			name: "有效格式 - GIF",
			parts: []ContentPart{
				{Type: "image_url", Data: "R0lGODlhAQABAAAA...", MIMEType: "image/gif"},
			},
			wantErr: nil,
		},
		{
			name: "大小写混合 - PNG",
			parts: []ContentPart{
				{Type: "image_url", Data: "iVBORw0KGgo...", MIMEType: "IMAGE/PNG"},
			},
			wantErr: nil,
		},
		{
			name: "大小写混合 - Jpeg",
			parts: []ContentPart{
				{Type: "image_url", Data: "/9j/4AAQSkZJRg...", MIMEType: "Image/Jpeg"},
			},
			wantErr: nil,
		},
		{
			name: "无效格式 - BMP",
			parts: []ContentPart{
				{Type: "image_url", Data: "Qk1...", MIMEType: "image/bmp"},
			},
			wantErr: ErrUnsupportedImageFormat,
		},
		{
			name: "无效格式 - SVG",
			parts: []ContentPart{
				{Type: "image_url", Data: "PHN2Zy...", MIMEType: "image/svg+xml"},
			},
			wantErr: ErrUnsupportedImageFormat,
		},
		{
			name: "空 Data",
			parts: []ContentPart{
				{Type: "image_url", Data: "", MIMEType: "image/png"},
			},
			wantErr: ErrEmptyImageData,
		},
		{
			name: "空 MIMEType",
			parts: []ContentPart{
				{Type: "image_url", Data: "iVBORw0KGgo...", MIMEType: ""},
			},
			wantErr: ErrUnsupportedImageFormat,
		},
		{
			name: "多个图片 - 都有效",
			parts: []ContentPart{
				{Type: "text", Text: "描述这些图片"},
				{Type: "image_url", Data: "iVBORw0KGgo...", MIMEType: "image/png"},
				{Type: "image_url", Data: "/9j/4AAQSkZJRg...", MIMEType: "image/jpeg"},
				{Type: "image_url", Data: "UklGRiQAAABXRUJQ...", MIMEType: "image/webp"},
			},
			wantErr: nil,
		},
		{
			name: "多个图片 - 第二个无效",
			parts: []ContentPart{
				{Type: "image_url", Data: "iVBORw0KGgo...", MIMEType: "image/png"},
				{Type: "image_url", Data: "Qk1...", MIMEType: "image/bmp"},
			},
			wantErr: ErrUnsupportedImageFormat,
		},
		{
			name: "纯文本 - 无需校验",
			parts: []ContentPart{
				{Type: "text", Text: "纯文本消息"},
			},
			wantErr: nil,
		},
		{
			name:    "空 Parts",
			parts:   []ContentPart{},
			wantErr: nil,
		},
		{
			name: "未识别 Type - 拼写错误 image",
			parts: []ContentPart{
				{Type: "image", Data: "iVBORw0KGgo...", MIMEType: "image/png"},
			},
			wantErr: ErrUnsupportedPartType,
		},
		{
			name: "预留 Type - video_url 占位，validate 放行",
			parts: []ContentPart{
				{Type: "video_url", Data: "/tmp/a.mp4"},
			},
			wantErr: nil,
		},
		{
			name: "预留 Type - audio_url 占位，validate 放行",
			parts: []ContentPart{
				{Type: "audio_url", Data: "/tmp/a.mp3"},
			},
			wantErr: nil,
		},
		{
			name: "未识别 Type - 空字符串",
			parts: []ContentPart{
				{Type: "", Text: "hi"},
			},
			wantErr: ErrUnsupportedPartType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateContentParts(tt.parts)
			if tt.wantErr == nil {
				if err != nil {
					t.Errorf("ValidateContentParts() error = %v, wantErr %v", err, tt.wantErr)
				}
			} else {
				if err == nil {
					t.Errorf("ValidateContentParts() error = nil, wantErr %v", tt.wantErr)
				} else if !errors.Is(err, tt.wantErr) {
					t.Errorf("ValidateContentParts() error = %v, wantErr %v", err, tt.wantErr)
				}
			}
		})
	}
}

func TestInferFilenameFromMIME(t *testing.T) {
	tests := []struct {
		name     string
		mimeType string
		want     string
		wantErr  error
	}{
		{
			name:     "PNG - 小写",
			mimeType: "image/png",
			want:     "image.png",
			wantErr:  nil,
		},
		{
			name:     "PNG - 大写",
			mimeType: "IMAGE/PNG",
			want:     "image.png",
			wantErr:  nil,
		},
		{
			name:     "JPEG - 小写",
			mimeType: "image/jpeg",
			want:     "image.jpg",
			wantErr:  nil,
		},
		{
			name:     "JPG - 小写",
			mimeType: "image/jpg",
			want:     "image.jpg",
			wantErr:  nil,
		},
		{
			name:     "JPEG - 大写",
			mimeType: "Image/JPEG",
			want:     "image.jpg",
			wantErr:  nil,
		},
		{
			name:     "WEBP - 小写",
			mimeType: "image/webp",
			want:     "image.webp",
			wantErr:  nil,
		},
		{
			name:     "GIF - 小写",
			mimeType: "image/gif",
			want:     "image.gif",
			wantErr:  nil,
		},
		{
			name:     "不支持 - BMP",
			mimeType: "image/bmp",
			want:     "",
			wantErr:  ErrUnsupportedImageFormat,
		},
		{
			name:     "不支持 - SVG",
			mimeType: "image/svg+xml",
			want:     "",
			wantErr:  ErrUnsupportedImageFormat,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := InferFilenameFromMIME(tt.mimeType)
			if tt.wantErr == nil {
				if err != nil {
					t.Errorf("InferFilenameFromMIME() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				matched, _ := regexp.MatchString(`^image_[0-9a-f]{8}\.(png|jpg|webp|gif)$`, got)
				if !matched {
					t.Errorf("InferFilenameFromMIME() = %v, want format image_<8hex>.<ext>", got)
				}
			} else {
				if err == nil {
					t.Errorf("InferFilenameFromMIME() error = nil, wantErr %v", tt.wantErr)
				} else if !errors.Is(err, tt.wantErr) {
					t.Errorf("InferFilenameFromMIME() error = %v, wantErr %v", err, tt.wantErr)
				}
			}
		})
	}
}
