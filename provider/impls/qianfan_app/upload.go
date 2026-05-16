package qianfan_app

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

// createConversation 创建新会话，返回 conversation_id。
// 仅在没有 SessionID 且需要上传图片时调用。
func createConversation(ctx context.Context, baseURL, apiKey, appID string) (string, error) {
	payload := map[string]any{
		"app_id": appID,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("qianfan_app: marshal conversation request: %w", err)
	}

	// baseURL 是 /v2/app/conversation/runs，需要改为 /v2/app/conversation
	conversationURL := strings.Replace(baseURL, "/runs", "", 1)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, conversationURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("qianfan_app: create conversation request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("qianfan_app: conversation request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("qianfan_app: read conversation response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("qianfan_app: create conversation failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		ConversationID string `json:"conversation_id"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("qianfan_app: parse conversation response: %w", err)
	}

	if result.ConversationID == "" {
		return "", fmt.Errorf("qianfan_app: conversation response missing conversation_id")
	}

	return result.ConversationID, nil
}

// uploadImages 并发上传多张图片到 Qianfan，返回 file_id 列表。
// 使用 sync.WaitGroup 实现并发上传，提高多图上传性能。
func uploadImages(ctx context.Context, baseURL, apiKey, appID, conversationID string, parts []base.ContentPart) ([]string, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("qianfan_app: upload baseURL is empty")
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

			fileID, err := uploadSingleImage(ctx, baseURL, apiKey, appID, conversationID, p)
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
			return nil, fmt.Errorf("qianfan_app: upload image[%d] failed: %w", result.index, result.err)
		}
		fileIDs[result.index] = result.fileID
	}

	return fileIDs, nil
}

// uploadSingleImage 上传单张图片到 Qianfan。
func uploadSingleImage(ctx context.Context, baseURL, apiKey, appID, conversationID string, part base.ContentPart) (string, error) {
	// 1. Base64 解码
	imageData, err := base64.StdEncoding.DecodeString(part.Data)
	if err != nil {
		return "", fmt.Errorf("qianfan_app: base64 decode failed: %w", err)
	}

	// 2. 构造 multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// 根据 MIME 类型推断文件扩展名
	filename, err := base.InferFilenameFromMIME(part.MIMEType)
	if err != nil {
		return "", fmt.Errorf("qianfan_app: %w", err)
	}

	// 添加文件字段
	fileWriter, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", fmt.Errorf("qianfan_app: create form file failed: %w", err)
	}

	if _, err := fileWriter.Write(imageData); err != nil {
		return "", fmt.Errorf("qianfan_app: write file data failed: %w", err)
	}

	// 添加其他字段
	if err := writer.WriteField("app_id", appID); err != nil {
		return "", fmt.Errorf("qianfan_app: write app_id field failed: %w", err)
	}

	if err := writer.WriteField("conversation_id", conversationID); err != nil {
		return "", fmt.Errorf("qianfan_app: write conversation_id field failed: %w", err)
	}

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("qianfan_app: close multipart writer failed: %w", err)
	}

	// 3. 发送上传请求
	// baseURL 是 /v2/app/conversation/runs，需要改为 /v2/app/conversation/file/upload
	uploadURL := strings.Replace(baseURL, "/runs", "/file/upload", 1)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, body)
	if err != nil {
		return "", fmt.Errorf("qianfan_app: create upload request failed: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("qianfan_app: upload request failed: %w", err)
	}
	defer resp.Body.Close()

	// 4. 解析响应获取 file_id
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
	if err != nil {
		return "", fmt.Errorf("qianfan_app: read upload response failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("qianfan_app: upload failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	// 响应格式：{"id": "file-xxx"}
	var uploadResp struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(respBody, &uploadResp); err != nil {
		return "", fmt.Errorf("qianfan_app: parse upload response failed: %w", err)
	}

	if uploadResp.ID == "" {
		return "", fmt.Errorf("qianfan_app: upload response missing file id")
	}

	return uploadResp.ID, nil
}
