package completions

type ApiReq struct {
	Messages        []ApiMessage `json:"messages"`
	Model           string       `json:"model"`
	Stream          bool         `json:"stream"`
	PluginIds       []string     `json:"plugin_ids"`
	ConversationId  string       `json:"conversation_id"`
	ParentMessageId string       `json:"parent_message_id"`
	NewMessages     string       `json:"-"`
}

type ApiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
