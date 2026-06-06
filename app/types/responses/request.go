package responses

type ApiReq struct {
	Model        string      `json:"model"`
	Input        interface{} `json:"input"`
	Instructions string      `json:"instructions"`
	Stream       bool        `json:"stream"`
	Tools        []Tool      `json:"tools"`
}

type Tool struct {
	Type string `json:"type"`
}
