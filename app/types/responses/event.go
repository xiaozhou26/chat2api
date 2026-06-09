package responses

import "encoding/json"

type Event struct {
	Type         string      `json:"type"`
	ResponseID   string      `json:"response_id,omitempty"`
	Response     *Response   `json:"response,omitempty"`
	OutputIndex  int         `json:"output_index,omitempty"`
	ContentIndex int         `json:"content_index,omitempty"`
	ItemID       string      `json:"item_id,omitempty"`
	Item         *OutputItem `json:"item,omitempty"`
	Delta        string      `json:"delta,omitempty"`
	Text         string      `json:"text,omitempty"`
	Name         string      `json:"name,omitempty"`
	CallID       string      `json:"call_id,omitempty"`
	Arguments    string      `json:"arguments,omitempty"`
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
	if event.Type == "" {
		return "data: " + string(data) + "\n\n"
	}
	return "event: " + event.Type + "\ndata: " + string(data) + "\n\n"
}
