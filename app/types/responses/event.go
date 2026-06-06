package responses

import "encoding/json"

type Event struct {
	Type         string      `json:"type"`
	Response     *Response   `json:"response,omitempty"`
	OutputIndex  int         `json:"output_index,omitempty"`
	ContentIndex int         `json:"content_index,omitempty"`
	ItemID       string      `json:"item_id,omitempty"`
	Item         *OutputItem `json:"item,omitempty"`
	Delta        string      `json:"delta,omitempty"`
	Text         string      `json:"text,omitempty"`
}

func CreatedEvent(responseID string, model string, created int64) Event {
	return Event{Type: "response.created", Response: &Response{
		ID:                responseID,
		Object:            "response",
		CreatedAt:         created,
		Status:            "in_progress",
		Error:             nil,
		IncompleteDetails: nil,
		Model:             model,
		Output:            []OutputItem{},
		ParallelToolCalls: false,
	}}
}

func CompletedEvent(responseID string, model string, created int64, output []OutputItem) Event {
	return Event{Type: "response.completed", Response: &Response{
		ID:                responseID,
		Object:            "response",
		CreatedAt:         created,
		Status:            "completed",
		Error:             nil,
		IncompleteDetails: nil,
		Model:             model,
		Output:            output,
		ParallelToolCalls: false,
	}}
}

func SSE(event Event) string {
	data, _ := json.Marshal(event)
	return "data: " + string(data) + "\n\n"
}
