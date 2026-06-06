package responses

type OutputItem struct {
	ID      string        `json:"id"`
	Type    string        `json:"type"`
	Status  string        `json:"status"`
	Role    string        `json:"role"`
	Content []ContentPart `json:"content"`
}

type ContentPart struct {
	Type        string        `json:"type"`
	Text        string        `json:"text"`
	Annotations []interface{} `json:"annotations"`
}

func TextOutputItem(id string, text string, status string) OutputItem {
	return OutputItem{
		ID:     id,
		Type:   "message",
		Status: status,
		Role:   "assistant",
		Content: []ContentPart{{
			Type:        "output_text",
			Text:        text,
			Annotations: []interface{}{},
		}},
	}
}
