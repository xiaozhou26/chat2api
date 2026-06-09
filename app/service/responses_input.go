package service

import (
	"chat2api/app/types/completions"
	"chat2api/app/types/responses"
	"encoding/json"
	"strings"
)

func completionMessagesFromResponse(req *responses.ApiReq) []completions.ApiMessage {
	messages := make([]completions.ApiMessage, 0)
	if strings.TrimSpace(req.Instructions) != "" {
		messages = append(messages, completions.ApiMessage{Role: "system", Content: strings.TrimSpace(req.Instructions)})
	}
	return append(messages, completionMessagesFromResponseInput(req.Input)...)
}

func completionMessagesFromResponseInput(input interface{}) []completions.ApiMessage {
	switch v := input.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return nil
		}
		return []completions.ApiMessage{{Role: "user", Content: strings.TrimSpace(v)}}
	case map[string]interface{}:
		return []completions.ApiMessage{{Role: responseStringValue(v["role"], "user"), Content: responseMessageContent(v)}}
	case []interface{}:
		messages := make([]completions.ApiMessage, 0, len(v))
		for _, item := range v {
			if part, ok := item.(map[string]interface{}); ok {
				content := responseMessageContent(part)
				if responseContentHasValue(content) {
					messages = append(messages, completions.ApiMessage{Role: responseStringValue(part["role"], "user"), Content: content})
				}
			}
		}
		return messages
	default:
		return nil
	}
}

func responseMessageContent(item map[string]interface{}) interface{} {
	if content, ok := item["content"].([]interface{}); ok {
		return content
	}
	return responseMessageContentText(item)
}

func responseContentHasValue(content interface{}) bool {
	switch v := content.(type) {
	case string:
		return strings.TrimSpace(v) != ""
	case []interface{}:
		return len(v) > 0
	default:
		return v != nil
	}
}

func responseMessageContentText(item map[string]interface{}) string {
	if text := responseStringValue(item["text"], ""); text != "" {
		return text
	}
	if text := responseStringValue(item["content"], ""); text != "" {
		return text
	}
	if content, ok := item["content"].([]interface{}); ok {
		parts := make([]string, 0, len(content))
		for _, raw := range content {
			if part, ok := raw.(map[string]interface{}); ok {
				if text := responseStringValue(part["text"], ""); text != "" {
					parts = append(parts, text)
				}
			}
		}
		return strings.Join(parts, "")
	}
	if isResponsesContentPart(item) {
		return responseStringValue(item["text"], "")
	}
	data, _ := json.Marshal(item)
	return string(data)
}

func isResponsesContentPart(item map[string]interface{}) bool {
	switch responseStringValue(item["type"], "") {
	case "text", "input_text", "output_text":
		return true
	default:
		return false
	}
}

func hasResponsesImageGenerationTool(req *responses.ApiReq) bool {
	if responseToolChoiceType(req.ToolChoice) == "image_generation" {
		return true
	}
	tools := req.Tools
	for _, tool := range tools {
		if strings.TrimSpace(tool.Type) == "image_generation" {
			return true
		}
	}
	return false
}

func completionToolsFromResponses(tools []responses.Tool) []completions.Tool {
	out := make([]completions.Tool, 0, len(tools))
	for _, tool := range tools {
		if strings.TrimSpace(tool.Type) != "function" {
			continue
		}
		out = append(out, completions.Tool{
			Type: "function",
			Function: completions.ToolFunction{
				Name:        strings.TrimSpace(tool.Name),
				Description: tool.Description,
				Parameters:  tool.Parameters,
				Strict:      tool.Strict,
			},
		})
	}
	return out
}

func completionToolChoiceFromResponses(value interface{}) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		if strings.TrimSpace(responseStringValue(v["type"], "")) == "function" {
			if _, ok := v["function"]; ok {
				return v
			}
			if name := strings.TrimSpace(responseStringValue(v["name"], "")); name != "" {
				return map[string]interface{}{
					"type":     "function",
					"function": map[string]interface{}{"name": name},
				}
			}
		}
	}
	return value
}

func responseToolChoiceType(value interface{}) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case map[string]interface{}:
		return strings.TrimSpace(responseStringValue(v["type"], ""))
	default:
		return ""
	}
}

func hasResponsesNonImageTools(tools []responses.Tool) bool {
	for _, tool := range tools {
		if strings.TrimSpace(tool.Type) != "" && strings.TrimSpace(tool.Type) != "image_generation" {
			return true
		}
	}
	return false
}

func responseStringValue(value interface{}, fallback string) string {
	if s, ok := value.(string); ok && strings.TrimSpace(s) != "" {
		return s
	}
	return fallback
}
