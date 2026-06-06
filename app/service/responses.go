package service

import (
	"chat2api/app/common"
	"chat2api/app/types/completions"
	"chat2api/app/types/responses"
	"chat2api/pkg/logx"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func Responses(c *gin.Context) {
	apiReq := &responses.ApiReq{}
	if err := c.BindJSON(apiReq); err != nil {
		common.ErrorResponse(c, http.StatusBadRequest, "Invalid parameter", nil)
		return
	}
	if hasResponsesImageGenerationTool(apiReq.Tools) {
		common.ErrorResponse(c, http.StatusNotImplemented, "responses image_generation tool is not implemented", nil)
		return
	}
	compReq := &completions.ApiReq{
		Model:    responses.NormalizeModel(apiReq.Model),
		Stream:   apiReq.Stream,
		Messages: completionMessagesFromResponse(apiReq),
	}
	if len(compReq.Messages) == 0 {
		common.ErrorResponse(c, http.StatusBadRequest, "input text is required", nil)
		return
	}
	result, err := runResponsesTextChat(c, compReq, apiReq.Stream)
	if err != nil {
		logx.WithContext(c.Request.Context()).Error(err.Error())
		common.ErrorResponse(c, http.StatusBadGateway, "", err.Error())
		return
	}
	if result == nil {
		return
	}
	item := responses.TextOutputItem(responses.MessageID(), result.Content, "completed")
	c.JSON(http.StatusOK, responses.CompletedEvent(responses.ResponseID(), compReq.Model, time.Now().Unix(), []responses.OutputItem{item}).Response)
}
