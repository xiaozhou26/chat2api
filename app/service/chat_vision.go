package service

import (
	"bytes"
	"chat2api/app/chatgpt_backend"
	"chat2api/app/types/chat"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/aurorax-neo/tls_client_httpi"
)

type uploadedImage struct {
	FileID   string
	FileName string
	FileSize int
	MimeType string
	Width    int
	Height   int
}

func prepareChatVisionInputs(backend *chatgpt_backend.Client, req *chat.Request) error {
	latestUserIndex := latestUserMessageIndex(req.Messages)
	for i := range req.Messages {
		message := &req.Messages[i]
		if message.Content.ContentType != "multimodal_text" {
			continue
		}
		parts := make([]interface{}, 0, len(message.Content.Parts))
		attachments := make([]chat.Attachment, 0)
		for _, part := range message.Content.Parts {
			imageValue := chatInputImageValue(part)
			if imageValue == "" {
				parts = append(parts, part)
				continue
			}
			if i != latestUserIndex {
				if text, ok := part.(string); ok && strings.TrimSpace(text) != "" {
					parts = append(parts, text)
				}
				continue
			}
			if pointer := existingImageAssetPointer(imageValue); pointer != "" {
				parts = append(parts, map[string]interface{}{
					"content_type":  "image_asset_pointer",
					"asset_pointer": pointer,
				})
				continue
			}
			if backend.AccAuth == "" {
				return fmt.Errorf("authenticated upstream account required for image input")
			}
			uploaded, err := uploadChatImage(backend, imageValue, fmt.Sprintf("image_%d.png", len(attachments)+1))
			if err != nil {
				return err
			}
			parts = append(parts, map[string]interface{}{
				"content_type":  "image_asset_pointer",
				"asset_pointer": fmt.Sprintf("file-service://%s", uploaded.FileID),
				"width":         uploaded.Width,
				"height":        uploaded.Height,
				"size_bytes":    uploaded.FileSize,
			})
			attachments = append(attachments, chat.Attachment{
				ID:       uploaded.FileID,
				MimeType: uploaded.MimeType,
				Name:     uploaded.FileName,
				Size:     uploaded.FileSize,
				Width:    uploaded.Width,
				Height:   uploaded.Height,
			})
		}
		message.Content.Parts = parts
		if len(attachments) > 0 {
			if message.Metadata == nil {
				message.Metadata = map[string]interface{}{}
			}
			message.Metadata["attachments"] = attachments
		}
	}
	return nil
}

func latestUserMessageIndex(messages []chat.Message) int {
	for i := len(messages) - 1; i >= 0; i-- {
		if strings.TrimSpace(messages[i].Author.Role) == "user" {
			return i
		}
	}
	return len(messages) - 1
}

func chatInputImageValue(part interface{}) string {
	item, ok := part.(map[string]interface{})
	if !ok {
		return ""
	}
	if pointer := existingImageAssetPointer(responseStringValue(item["asset_pointer"], "")); pointer != "" {
		return pointer
	}
	partType := strings.TrimSpace(responseStringValue(item["type"], ""))
	contentType := strings.TrimSpace(responseStringValue(item["content_type"], ""))
	if partType != "input_image" && partType != "image" && partType != "image_url" && contentType != "image_asset_pointer" && item["image_url"] == nil {
		return ""
	}
	return responseImageValue(item)
}

func existingImageAssetPointer(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "file-service://") || strings.HasPrefix(value, "sediment://") {
		return value
	}
	return ""
}

func uploadChatImage(backend *chatgpt_backend.Client, value string, fileName string) (uploadedImage, error) {
	data, mimeType, resolvedName, err := decodeInputImage(value, fileName)
	if err != nil {
		return uploadedImage{}, err
	}
	width, height, err := imageSize(data)
	if err != nil {
		return uploadedImage{}, err
	}
	path := "/backend-api/files"
	metaReq := map[string]interface{}{
		"file_name": resolvedName,
		"file_size": len(data),
		"use_case":  "multimodal",
		"width":     width,
		"height":    height,
	}
	metaBody, _ := json.Marshal(metaReq)
	headers, cookies := backend.Headers(backend.BaseURL + path)
	headers.Set("accept", "application/json")
	headers.Set("content-type", "application/json")
	resp, err := backend.HTTP.Request(tls_client_httpi.POST, backend.BaseURL+path, headers, cookies, bytes.NewReader(metaBody))
	if err != nil {
		return uploadedImage{}, err
	}
	defer resp.Body.Close()
	if !isHTTPSuccess(resp.StatusCode) {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return uploadedImage{}, fmt.Errorf("image upload init failed: status=%d body=%s", resp.StatusCode, string(body))
	}
	var meta struct {
		UploadURL string `json:"upload_url"`
		FileID    string `json:"file_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return uploadedImage{}, err
	}
	if meta.UploadURL == "" || meta.FileID == "" {
		return uploadedImage{}, fmt.Errorf("image upload init returned incomplete metadata")
	}
	putHeaders := tls_client_httpi.Headers{}
	putHeaders.Set("content-type", mimeType)
	putHeaders.Set("x-ms-blob-type", "BlockBlob")
	putHeaders.Set("x-ms-version", "2020-04-08")
	putHeaders.Set("origin", backend.BaseURL)
	putHeaders.Set("referer", backend.BaseURL+"/")
	putHeaders.Set("user-agent", backend.UserAgent)
	putHeaders.Set("accept", "application/json, text/plain, */*")
	putHeaders.Set("accept-language", "en-US,en;q=0.8")
	putResp, err := backend.HTTP.Request(tls_client_httpi.PUT, meta.UploadURL, putHeaders, nil, bytes.NewReader(data))
	if err != nil {
		return uploadedImage{}, err
	}
	defer putResp.Body.Close()
	if !isHTTPSuccess(putResp.StatusCode) {
		body, _ := io.ReadAll(io.LimitReader(putResp.Body, 4096))
		return uploadedImage{}, fmt.Errorf("image upload failed: status=%d body=%s", putResp.StatusCode, string(body))
	}
	finishPath := fmt.Sprintf("/backend-api/files/%s/uploaded", meta.FileID)
	finishHeaders, finishCookies := backend.Headers(backend.BaseURL + finishPath)
	finishHeaders.Set("accept", "application/json")
	finishHeaders.Set("content-type", "application/json")
	finishResp, err := backend.HTTP.Request(tls_client_httpi.POST, backend.BaseURL+finishPath, finishHeaders, finishCookies, bytes.NewBufferString("{}"))
	if err != nil {
		return uploadedImage{}, err
	}
	defer finishResp.Body.Close()
	if !isHTTPSuccess(finishResp.StatusCode) {
		body, _ := io.ReadAll(io.LimitReader(finishResp.Body, 4096))
		return uploadedImage{}, fmt.Errorf("image upload finalize failed: status=%d body=%s", finishResp.StatusCode, string(body))
	}
	return uploadedImage{FileID: meta.FileID, FileName: resolvedName, FileSize: len(data), MimeType: mimeType, Width: width, Height: height}, nil
}

func decodeInputImage(value string, fallbackName string) ([]byte, string, string, error) {
	value = strings.TrimSpace(value)
	fileName := fallbackName
	if value != "" && len(value) < 512 && !strings.ContainsAny(value, "\r\n") && !strings.HasPrefix(value, "data:") && !strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://") {
		path := filepath.Clean(value)
		if data, err := os.ReadFile(path); err == nil {
			mimeType := http.DetectContentType(data)
			return data, normalizeImageMime(mimeType), filepath.Base(path), nil
		}
	}
	mimeType := "image/png"
	payload := value
	if strings.HasPrefix(value, "data:") {
		header, body, ok := strings.Cut(value, ",")
		if !ok {
			return nil, "", "", fmt.Errorf("invalid image data url")
		}
		payload = body
		mimeType = strings.TrimPrefix(strings.Split(strings.TrimPrefix(header, "data:"), ";")[0], " ")
	}
	data, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return nil, "", "", fmt.Errorf("invalid base64 image data: %w", err)
	}
	if mimeType == "" || mimeType == "application/octet-stream" {
		mimeType = http.DetectContentType(data)
	}
	return data, normalizeImageMime(mimeType), fileName, nil
}

func imageSize(data []byte) (int, int, error) {
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0, fmt.Errorf("decode image size failed: %w", err)
	}
	return config.Width, config.Height, nil
}

func normalizeImageMime(mimeType string) string {
	mimeType = strings.ToLower(strings.TrimSpace(strings.Split(mimeType, ";")[0]))
	if mimeType == "image/jpg" {
		return "image/jpeg"
	}
	if strings.HasPrefix(mimeType, "image/") {
		return mimeType
	}
	return "image/png"
}

func isHTTPSuccess(status int) bool {
	return status >= 200 && status < 300
}
