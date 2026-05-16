package openai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"sync"

	"github.com/Michaelxwb/ai-api-sdk/provider/base"
)

// uploadImagesForMoonshot 为 Moonshot 供应商上传图片，返回 file_id 列表。
// 使用 sync.WaitGroup 实现并发上传，提高多图上传性能。
//
// 注意：此函数专为 Moonshot 设计。
// - Moonshot 与 OpenAI 在纯文本对话时的请求/响应格式完全相同
// - 但在处理图片时，OpenAI 使用 base64 内联，Moonshot 使用文件上传（ms://file_id 格式）
// - 其他 OpenAI 兼容供应商（FastGPT/Ollama/Bailian）继续使用 base64 内联方式
//
// 未来优化：如需拆分 Moonshot 为独立供应商实现，可将此函数迁移至 provider/impls/moonshot/upload.go。
func uploadImagesForMoonshot(ctx context.Context, baseURL, apiKey string, parts []base.ContentPart) ([]string, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("moonshot: upload baseURL is empty")
	}

	// 提取所有 image_url 类型的 part
	var imageParts []base.ContentPart
	for _, part := range parts {
		if part.Type == "image_url" {
			imageParts = append(imageParts, part)
		}
	}

	if len(imageParts) == 0 {
		return nil, nil
	}

	// 并发上传
	type uploadResult struct {
		fileID string
		err    error
		index  int
	}

	results := make(chan uploadResult, len(imageParts))
	var wg sync.WaitGroup

	for i, part := range imageParts {
		wg.Add(1)
		go func(idx int, p base.ContentPart) {
			defer wg.Done()

			fileID, err := uploadSingleImageForMoonshot(ctx, baseURL, apiKey, p)
			results <- uploadResult{
				fileID: fileID,
				err:    err,
				index:  idx,
			}
		}(i, part)
	}

	// 等待所有上传完成
	wg.Wait()
	close(results)

	// 收集结果（保持顺序）
	fileIDs := make([]string, len(imageParts))
	for result := range results {
		if result.err != nil {
			return nil, fmt.Errorf("moonshot: upload image[%d] failed: %w", result.index, result.err)
		}
		fileIDs[result.index] = result.fileID
	}

	return fileIDs, nil
}

// uploadSingleImageForMoonshot 上传单张图片到 Moonshot。
func uploadSingleImageForMoonshot(ctx context.Context, baseURL, apiKey string, part base.ContentPart) (string, error) {
	// 1. Base64 解码
	imageData, err := base64.StdEncoding.DecodeString(part.Data)
	if err != nil {
		return "", fmt.Errorf("moonshot: base64 decode failed: %w", err)
	}

	// 2. 构造 multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// 根据 MIME 类型推断文件扩展名
	filename, err := base.InferFilenameFromMIME(part.MIMEType)
	if err != nil {
		return "", fmt.Errorf("moonshot: %w", err)
	}

	// 添加文件字段
	fileWriter, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", fmt.Errorf("moonshot: create form file failed: %w", err)
	}

	if _, err := fileWriter.Write(imageData); err != nil {
		return "", fmt.Errorf("moonshot: write file data failed: %w", err)
	}

	// 添加 purpose 字段（Moonshot 要求）
	if err := writer.WriteField("purpose", "image"); err != nil {
		return "", fmt.Errorf("moonshot: write purpose field failed: %w", err)
	}

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("moonshot: close multipart writer failed: %w", err)
	}

	// 3. 发送上传请求
	url := strings.TrimRight(baseURL, "/") + "/files"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return "", fmt.Errorf("moonshot: create upload request failed: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("moonshot: upload request failed: %w", err)
	}
	defer resp.Body.Close()

	// 4. 解析响应获取 file_id
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
	if err != nil {
		return "", fmt.Errorf("moonshot: read upload response failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("moonshot: upload failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	// Moonshot 响应格式：{"id": "file-xxx", ...}
	var uploadResp struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(respBody, &uploadResp); err != nil {
		return "", fmt.Errorf("moonshot: parse upload response failed: %w", err)
	}

	if uploadResp.ID == "" {
		return "", fmt.Errorf("moonshot: upload response missing file id")
	}

	return uploadResp.ID, nil
}
