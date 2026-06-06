package chat

import "time"

type Response struct {
	Message struct {
		Id     string `json:"id"`
		Author struct {
			Role     string      `json:"role"`
			Name     interface{} `json:"name"`
			Metadata struct {
			} `json:"metadata"`
		} `json:"author"`
		CreateTime float64     `json:"create_time"`
		UpdateTime interface{} `json:"update_time"`
		Content    struct {
			ContentType string        `json:"content_type"`
			Parts       []interface{} `json:"parts"`
			Language    string        `json:"language"`
			Text        string        `json:"text"`
		} `json:"content"`
		Status    string      `json:"status"`
		EndTurn   interface{} `json:"end_turn"`
		Weight    float64     `json:"weight"`
		Metadata  Metadata    `json:"metadata"`
		Recipient string      `json:"recipient"`
	} `json:"message"`
	ConversationId     string      `json:"conversation_id"`
	Error              interface{} `json:"error"`
	Type               string      `json:"types"`
	MessageId          string      `json:"message_id"`
	IsCompletion       bool        `json:"is_completion"`
	ModerationResponse struct {
		Flagged      bool          `json:"flagged"`
		Disclaimers  []interface{} `json:"disclaimers"`
		Blocked      bool          `json:"blocked"`
		ModerationId string        `json:"moderation_id"`
	} `json:"moderation_response"`
}

type Info struct {
	Type         string    `json:"types"`
	CallToAction string    `json:"call_to_action"`
	ResetsAfter  time.Time `json:"resets_after"`
	LimitDetails struct {
		Type                  string `json:"types"`
		ModelSlug             string `json:"model_slug"`
		UsingDefaultModelSlug string `json:"using_default_model_slug"`
		NextModelSlug         string `json:"next_model_slug"`
		ModelLimitName        string `json:"model_limit_name"`
	} `json:"limit_details"`
	DisplayDescription struct {
		Type                string      `json:"types"`
		Description         string      `json:"description"`
		MarkdownDescription interface{} `json:"markdown_description"`
	} `json:"display_description"`
	ConversationId string `json:"conversation_id"`
}
