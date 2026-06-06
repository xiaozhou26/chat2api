package service

import (
	"chat2api/app/chatgpt_backend"
	"chat2api/app/common"
	"chat2api/app/types/chat"
	"fmt"
	"net/http"

	"github.com/aurorax-neo/tls_client_httpi"
	"github.com/gin-gonic/gin"
)

func sendChatRequest(c *gin.Context, chatReq *chat.Request) (*http.Response, string, error) {
	body, err := common.Struct2BytesBuffer(chatReq)
	if err != nil {
		return nil, "", err
	}
	backend, err := chatgpt_backend.New(c.Request.Header.Get("Authorization"), chatgpt_backend.Retry())
	if err != nil {
		return nil, "", err
	}
	headers, cookies := backend.Headers(backend.ChatURL)
	headers.Set("accept", "text/event-stream")
	headers.Set("content-type", "application/json")
	headers.Set("openai-sentinel-chat-requirements-token", backend.Auth.Token)
	if backend.Auth.ProofWork.Ospt != "" {
		headers.Set("openai-sentinel-proof-token", backend.Auth.ProofWork.Ospt)
	}
	if backend.Auth.TurnstileToken != "" {
		headers.Set("openai-sentinel-turnstile-token", backend.Auth.TurnstileToken)
	}
	if backend.Auth.SoToken != "" {
		headers.Set("openai-sentinel-so-token", backend.Auth.SoToken)
	}
	response, err := backend.HTTP.Request(tls_client_httpi.POST, backend.ChatURL, headers, cookies, body)
	if err != nil {
		return nil, backend.AccAuth, fmt.Errorf("upstream request failed: %w", err)
	}
	return response, backend.AccAuth, nil
}
