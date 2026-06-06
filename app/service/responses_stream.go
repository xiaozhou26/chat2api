package service

import (
	"chat2api/app/types/responses"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func streamResponsesTextEvents(c *gin.Context, model string, resp *http.Response) (*chatResult, error) {
	c.Header("Content-Type", "text/event-stream")
	responseID := responses.ResponseID()
	itemID := responses.MessageID()
	created := time.Now().Unix()
	if _, err := c.Writer.WriteString(responses.SSE(responses.CreatedEvent(responseID, model, created))); err != nil {
		return nil, err
	}
	item := responses.TextOutputItem("", "", "in_progress")
	item.ID = itemID
	if _, err := c.Writer.WriteString(responses.SSE(responses.Event{Type: "response.output_item.added", OutputIndex: 0, Item: &item})); err != nil {
		return nil, err
	}
	c.Writer.Flush()
	result, err := handleChatStream(resp, func(event chatStreamEvent) error {
		if event.Delta == "" {
			return nil
		}
		if _, err := c.Writer.WriteString(responses.SSE(responses.Event{Type: "response.output_text.delta", ItemID: itemID, OutputIndex: 0, ContentIndex: 0, Delta: event.Delta})); err != nil {
			return err
		}
		c.Writer.Flush()
		return nil
	})
	if err != nil {
		return nil, err
	}
	if _, err := c.Writer.WriteString(responses.SSE(responses.Event{Type: "response.output_text.done", ItemID: itemID, OutputIndex: 0, ContentIndex: 0, Text: result.Content})); err != nil {
		return nil, err
	}
	completedItem := responses.TextOutputItem(itemID, result.Content, "completed")
	if _, err := c.Writer.WriteString(responses.SSE(responses.Event{Type: "response.output_item.done", OutputIndex: 0, Item: &completedItem})); err != nil {
		return nil, err
	}
	if _, err := c.Writer.WriteString(responses.SSE(responses.CompletedEvent(responseID, model, created, []responses.OutputItem{completedItem}))); err != nil {
		return nil, err
	}
	_, _ = c.Writer.WriteString("data: [DONE]\n\n")
	c.Writer.Flush()
	return result, nil
}
