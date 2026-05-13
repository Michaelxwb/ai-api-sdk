package coze

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

// uploadImages 并发上传多张图片到 Coze，返回 file_id 列表。
// 使用 sync.WaitGroup 实现并发上传，提高多图上传性能。
func uploadImages(ctx context.Context, baseURL, apiKey string, parts []base.ContentPart) ([]string, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("coze: upload baseURL is empty")
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

			fileID, err := uploadSingleImage(ctx, baseURL, apiKey, p)
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
			return nil, fmt.Errorf("coze: upload image[%d] failed: %w", result.index, result.err)
		}
		fileIDs[result.index] = result.fileID
	}

	return fileIDs, nil
}

// uploadSingleImage 上传单张图片到 Coze。
func uploadSingleImage(ctx context.Context, baseURL, apiKey string, part base.ContentPart) (string, error) {
	// 1. Base64 解码
	imageData, err := base64.StdEncoding.DecodeString(part.Data)
	if err != nil {
		return "", fmt.Errorf("coze: base64 decode failed: %w", err)
	}

	// 2. 构造 multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// 根据 MIME 类型推断文件扩展名
	filename, err := base.InferFilenameFromMIME(part.MIMEType)
	if err != nil {
		return "", fmt.Errorf("coze: %w", err)
	}

	// 添加文件字段
	fileWriter, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", fmt.Errorf("coze: create form file failed: %w", err)
	}

	if _, err := fileWriter.Write(imageData); err != nil {
		return "", fmt.Errorf("coze: write file data failed: %w", err)
	}

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("coze: close multipart writer failed: %w", err)
	}

	// 3. 发送上传请求
	url := strings.Replace(baseURL, "/v3", "/v1", 1) + "/files/upload"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return "", fmt.Errorf("coze: create upload request failed: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("coze: upload request failed: %w", err)
	}
	defer resp.Body.Close()

	// 4. 解析响应获取 file_id
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
	if err != nil {
		return "", fmt.Errorf("coze: read upload response failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("coze: upload failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	// 响应格式：{"code":0,"data":{"id":"7634013053008937000",...}}
	var uploadResp struct {
		Code int `json:"code"`
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
		Msg string `json:"msg"`
	}
	if err := json.Unmarshal(respBody, &uploadResp); err != nil {
		return "", fmt.Errorf("coze: parse upload response failed: %w", err)
	}

	if uploadResp.Code != 0 {
		return "", fmt.Errorf("coze: upload failed with code %d: %s", uploadResp.Code, uploadResp.Msg)
	}

	if uploadResp.Data.ID == "" {
		return "", fmt.Errorf("coze: upload response missing file id")
	}

	return uploadResp.Data.ID, nil
}
