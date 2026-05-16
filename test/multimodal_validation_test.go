package test

import (
	"context"
	"errors"
	"testing"

	"github.com/Michaelxwb/ai-api-sdk/client"
	"github.com/Michaelxwb/ai-api-sdk/provider/base"
)

// TestMultimodalBackwardCompatibility 测试向后兼容：len(Parts)==0 时使用 Content
func TestMultimodalBackwardCompatibility(t *testing.T) {
	c := client.New()

	// 使用 Quick API 创建 session（不会真正调用API，只测试校验逻辑）
	qs, err := c.Quick(client.ProviderConfig{
		Provider: "openai",
		APIKey:   "sk-test",
		Model:    "gpt-4",
	})

	if err != nil {
		t.Fatalf("创建 session 失败: %v", err)
	}

	// 纯文本消息（Parts 为空）
	_, err = qs.Send(context.Background(), []base.Message{
		{Role: "user", Content: "纯文本消息", Parts: nil},
	})

	// 不期望校验错误（期望的错误是网络错误，不是校验错误）
	if err != nil && (errors.Is(err, base.ErrUnsupportedImageFormat) || errors.Is(err, base.ErrEmptyImageData)) {
		t.Errorf("纯文本请求不应该触发校验错误: %v", err)
	}
}

// TestMultimodalValidationErrors 测试多模态校验错误
func TestMultimodalValidationErrors(t *testing.T) {
	c := client.New()

	qs, err := c.Quick(client.ProviderConfig{
		Provider: "openai",
		APIKey:   "sk-test",
		Model:    "gpt-4",
	})

	if err != nil {
		t.Fatalf("创建 session 失败: %v", err)
	}

	tests := []struct {
		name    string
		parts   []base.ContentPart
		wantErr error
	}{
		{
			name: "有效的图片 - 应通过校验",
			parts: []base.ContentPart{
				{Type: "text", Text: "描述这张图片"},
				{Type: "image_url", Data: "iVBORw0KGgo...", MIMEType: "image/png"},
			},
			wantErr: nil, // 期望通过校验（可能因网络失败，但不是校验错误）
		},
		{
			name: "无效的图片格式 - BMP",
			parts: []base.ContentPart{
				{Type: "image_url", Data: "Qk1...", MIMEType: "image/bmp"},
			},
			wantErr: base.ErrUnsupportedImageFormat,
		},
		{
			name: "空图片数据",
			parts: []base.ContentPart{
				{Type: "image_url", Data: "", MIMEType: "image/png"},
			},
			wantErr: base.ErrEmptyImageData,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := qs.Send(context.Background(), []base.Message{
				{Role: "user", Parts: tt.parts},
			})

			if tt.wantErr == nil {
				// 有效的 Parts 应该通过校验（可能会因为网络等其他原因失败，但不应该是校验错误）
				if err != nil && (errors.Is(err, base.ErrUnsupportedImageFormat) || errors.Is(err, base.ErrEmptyImageData)) {
					t.Errorf("有效的 Parts 不应该触发校验错误: %v", err)
				}
			} else {
				// 无效的 Parts 应该返回校验错误
				if err == nil {
					t.Errorf("期望返回校验错误 %v，但没有错误", tt.wantErr)
				} else if !errors.Is(err, tt.wantErr) {
					t.Errorf("期望错误 %v，实际错误 %v", tt.wantErr, err)
				}
			}
		})
	}
}

// TestMultimodalValidationStream 测试流式接口的多模态校验
func TestMultimodalValidationStream(t *testing.T) {
	c := client.New()

	qs, err := c.Quick(client.ProviderConfig{
		Provider: "openai",
		APIKey:   "sk-test",
		Model:    "gpt-4",
	})

	if err != nil {
		t.Fatalf("创建 session 失败: %v", err)
	}

	// Send 方法会调用内部的 Chat 或 ChatStream，都会触发校验
	_, err = qs.Send(context.Background(), []base.Message{
		{Role: "user", Parts: []base.ContentPart{
			{Type: "image_url", Data: "", MIMEType: "image/png"}, // 空数据
		}},
	})

	if err == nil {
		t.Error("期望返回校验错误，但没有错误")
	} else if !errors.Is(err, base.ErrEmptyImageData) {
		t.Errorf("期望错误 %v，实际错误 %v", base.ErrEmptyImageData, err)
	}
}
