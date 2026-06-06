package service

import (
	"bufio"
	"bytes"
	"chat2api/app/common"
	"chat2api/app/token_pool"
	"chat2api/app/types/chat"
	"chat2api/app/types/completions"
	"chat2api/pkg/logx"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func Completions(c *gin.Context) {
	apiReq := &completions.ApiReq{}
	err := c.BindJSON(apiReq)
	if err != nil {
		common.ErrorResponse(c, http.StatusBadRequest, "Invalid parameter", nil)
		return
	}
	chatReq := completions.BuildChatRequest(apiReq)
	if chatReq.Model == "" {
		errStr := fmt.Sprint("Model is unsupported")
		logx.WithContext(c.Request.Context()).Error(errStr)
		common.ErrorResponse(c, http.StatusBadRequest, errStr, nil)
		return
	}
	response, accessToken, err := sendChatRequest(c, chatReq)
	if err != nil {
		logx.WithContext(c.Request.Context()).Error(err.Error())
		common.ErrorResponse(c, http.StatusBadGateway, "upstream request failed", err.Error())
		return
	}
	defer response.Body.Close()
	if handleResponseError(c, response, accessToken) {
		return
	}
	result, err := handlerResponse(c, apiReq, response)
	if err != nil {
		logx.WithContext(c.Request.Context()).Error(err.Error())
		common.ErrorResponse(c, http.StatusBadGateway, "", err.Error())
		return
	}
	if !apiReq.Stream {
		resp := completions.NewApiRespJson(completions.GenerateCompletionID(29), apiReq.Model, result.Content)
		resp.ConversationId = result.ConversationId
		resp.MessageId = result.MessageId
		c.JSON(http.StatusOK, resp)
	}
}

func handleResponseError(c *gin.Context, response *http.Response, accessToken string) bool {
	if response.StatusCode == http.StatusOK {
		return false
	}
	body, _ := io.ReadAll(io.LimitReader(response.Body, 64*1024))
	if response.StatusCode == http.StatusTooManyRequests {
		canUseAt := rateLimitCanUseAt(response, body)
		token_pool.GetAccessTokenPool().SetCanUseAt(accessToken, canUseAt)
	}
	var errorResponse map[string]interface{}
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&errorResponse); err != nil {
		common.ErrorResponse(c, response.StatusCode, "Unknown error", errors.New(string(body)))
		return true
	}
	common.ErrorResponse(c, response.StatusCode, errorResponse["detail"], nil)
	return true
}

func rateLimitCanUseAt(response *http.Response, body []byte) int64 {
	now := time.Now()
	if value := parseRetryAfter(response.Header.Get("Retry-After"), now); value > 0 {
		return value
	}
	if value := parseRateLimitBody(body, now); value > 0 {
		return value
	}
	return now.Add(time.Hour).Unix()
}

func parseRetryAfter(value string, now time.Time) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil {
		if seconds < 0 {
			seconds = 0
		}
		return now.Add(time.Duration(seconds) * time.Second).Unix()
	}
	if t, err := http.ParseTime(value); err == nil {
		return t.Unix()
	}
	return 0
}

func parseRateLimitBody(body []byte, now time.Time) int64 {
	if len(body) == 0 {
		return 0
	}
	var payload interface{}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return 0
	}
	return findRateLimitTime(payload, now)
}

func findRateLimitTime(value interface{}, now time.Time) int64 {
	switch v := value.(type) {
	case map[string]interface{}:
		for _, key := range []string{"retry_after", "reset_after", "resets_after", "restore_at", "reset_at"} {
			if candidate, ok := v[key]; ok {
				if parsed := parseRateLimitValue(candidate, now); parsed > 0 {
					return parsed
				}
			}
		}
		for _, child := range v {
			if parsed := findRateLimitTime(child, now); parsed > 0 {
				return parsed
			}
		}
	case []interface{}:
		for _, child := range v {
			if parsed := findRateLimitTime(child, now); parsed > 0 {
				return parsed
			}
		}
	}
	return 0
}

func parseRateLimitValue(value interface{}, now time.Time) int64 {
	switch v := value.(type) {
	case json.Number:
		if seconds, err := v.Int64(); err == nil {
			return normalizeRateLimitUnix(seconds, now)
		}
		if f, err := v.Float64(); err == nil {
			return normalizeRateLimitUnix(int64(f), now)
		}
	case float64:
		return normalizeRateLimitUnix(int64(v), now)
	case string:
		return parseRateLimitString(v, now)
	}
	return 0
}

func normalizeRateLimitUnix(value int64, now time.Time) int64 {
	if value <= 0 {
		return 0
	}
	if value < 30*24*3600 {
		return now.Add(time.Duration(value) * time.Second).Unix()
	}
	return value
}

func parseRateLimitString(value string, now time.Time) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil {
		return normalizeRateLimitUnix(seconds, now)
	}
	if duration, err := time.ParseDuration(value); err == nil {
		return now.Add(duration).Unix()
	}
	for _, layout := range []string{time.RFC3339, time.RFC3339Nano, time.DateTime, "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, value); err == nil {
			return t.Unix()
		}
	}
	if t, err := http.ParseTime(value); err == nil {
		return t.Unix()
	}
	return 0
}

type chatResult struct {
	Content        string
	ConversationId string
	MessageId      string
	FinishReason   string
}

type chatStreamEvent struct {
	Response     chat.Response
	Delta        string
	IsFirstChunk bool
	Result       *chatResult
}

func handleChatStream(resp *http.Response, onEvent func(chatStreamEvent) error) (*chatResult, error) {
	reader := bufio.NewReader(resp.Body)
	var previousText chat.StringStruct
	isFirstChunk := true
	result := &chatResult{}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data: "))
		if payload == "" {
			continue
		}
		if payload == "[DONE]" {
			break
		}
		var chatResp chat.Response
		if err := json.Unmarshal([]byte(payload), &chatResp); err != nil {
			continue
		}
		if chatResp.Error != nil {
			return nil, fmt.Errorf("chatgpt error: %v", chatResp.Error)
		}
		if chatResp.Message.Author.Role != "assistant" || chatResp.Message.Content.Parts == nil {
			continue
		}
		if chatResp.ConversationId != "" {
			result.ConversationId = chatResp.ConversationId
		}
		if chatResp.Message.Id != "" {
			result.MessageId = chatResp.Message.Id
		}
		if chatResp.Message.Metadata.MessageType != "" &&
			chatResp.Message.Metadata.MessageType != "next" &&
			chatResp.Message.Metadata.MessageType != "continue" {
			continue
		}
		if chatResp.Message.Content.ContentType != "" && !strings.HasSuffix(chatResp.Message.Content.ContentType, "text") {
			continue
		}
		if len(chatResp.Message.Content.Parts) == 0 {
			continue
		}
		text, ok := chatResp.Message.Content.Parts[0].(string)
		if !ok {
			continue
		}
		delta := completions.DeltaText(text, previousText.Text)
		if !isFirstChunk && delta == "" {
			continue
		}
		previousText.Text = text
		if onEvent != nil {
			if err := onEvent(chatStreamEvent{
				Response:     chatResp,
				Delta:        delta,
				IsFirstChunk: isFirstChunk,
				Result:       result,
			}); err != nil {
				return nil, err
			}
		}
		isFirstChunk = false
		if chatResp.Message.Metadata.FinishDetails != nil {
			result.FinishReason = chatResp.Message.Metadata.FinishDetails.Type
		}
	}
	result.Content = previousText.Text
	return result, nil
}

func handlerResponse(c *gin.Context, apiReq *completions.ApiReq, resp *http.Response) (*chatResult, error) {
	if apiReq.Stream {
		c.Header("Content-Type", "text/event-stream")
	} else {
		c.Header("Content-Type", "application/json")
	}
	id := completions.GenerateCompletionID(29)
	result, err := handleChatStream(resp, func(event chatStreamEvent) error {
		if !apiReq.Stream {
			return nil
		}
		apiRespJson := completions.NewApiRespStream(id, apiReq.Model, event.Delta)
		apiRespJson.ConversationId = event.Response.ConversationId
		apiRespJson.MessageId = event.Response.Message.Id
		if event.IsFirstChunk {
			apiRespJson.Choices[0].Delta.Role = event.Response.Message.Author.Role
		}
		if _, err := c.Writer.WriteString("data: " + apiRespJson.String() + "\n\n"); err != nil {
			return err
		}
		c.Writer.Flush()
		return nil
	})
	if err != nil {
		return nil, err
	}
	if apiReq.Stream {
		finalLine := completions.StopChunk(id, apiReq.Model, result.FinishReason)
		_, _ = c.Writer.WriteString(fmt.Sprint("data: ", finalLine.String(), "\n\n"))
		_, _ = c.Writer.WriteString("data: [DONE]\n\n")
	}
	return result, nil
}
