package completions

import (
	"encoding/json"
)

type ApiRespStream struct {
	ID             string            `json:"id,omitempty"`
	Object         string            `json:"object,omitempty"`
	Created        int64             `json:"created,omitempty"`
	Model          string            `json:"model,omitempty"`
	ConversationId string            `json:"conversation_id,omitempty"`
	MessageId      string            `json:"message_id,omitempty"`
	Choices        []ApiStreamChoice `json:"choices,omitempty"`
}

type ApiStreamChoice struct {
	Delta        ApiStreamDelta `json:"delta,omitempty"`
	Index        int            `json:"index,omitempty"`
	FinishReason interface{}    `json:"finish_reason,omitempty"`
}

type ApiStreamDelta struct {
	Content string `json:"content,omitempty"`
	Role    string `json:"role,omitempty"`
}

func (ARS *ApiRespStream) String() string {
	resp, _ := json.Marshal(ARS)
	return string(resp)
}
