package service

import (
	"chat2api/app/types/completions"

	"github.com/gin-gonic/gin"
)

func runResponsesTextChat(c *gin.Context, apiReq *completions.ApiReq, streamResponses bool) (*chatResult, error) {
	chatReq := completions.BuildChatRequest(apiReq)
	resp, accessToken, err := sendChatRequest(c, chatReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if handleResponseError(c, resp, accessToken) {
		return nil, nil
	}
	if streamResponses {
		return streamResponsesTextEvents(c, apiReq.Model, resp)
	}
	return handlerResponse(c, apiReq, resp)
}
