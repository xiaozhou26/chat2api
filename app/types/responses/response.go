package responses

type Response struct {
	ID                string       `json:"id"`
	Object            string       `json:"object"`
	CreatedAt         int64        `json:"created_at"`
	Status            string       `json:"status"`
	Error             interface{}  `json:"error"`
	IncompleteDetails interface{}  `json:"incomplete_details"`
	Model             string       `json:"model"`
	Output            []OutputItem `json:"output"`
	ParallelToolCalls bool         `json:"parallel_tool_calls"`
}
