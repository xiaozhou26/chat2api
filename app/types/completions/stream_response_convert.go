package completions

import (
	"chat2api/app/types/chat"
	"encoding/json"
)

func ConvertToString(id string, model string, chatResp *chat.Response, previousText *chat.StringStruct, role bool) string {
	if len(chatResp.Message.Content.Parts) == 0 {
		return ""
	}
	text, ok := chatResp.Message.Content.Parts[0].(string)
	if !ok {
		return ""
	}
	apiRespJson := NewApiRespStream(id, model, DeltaText(text, previousText.Text))
	apiRespJson.ConversationId = chatResp.ConversationId
	apiRespJson.MessageId = chatResp.Message.Id
	if role {
		apiRespJson.Choices[0].Delta.Role = chatResp.Message.Author.Role
	} else if apiRespJson.Choices[0].Delta.Content == "" {
		return ""
	}
	previousText.Text = text
	data, _ := json.Marshal(apiRespJson)
	return "data: " + string(data) + "\n\n"
}
