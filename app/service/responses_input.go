package service

import (
	"chat2api/app/types/completions"
	"chat2api/app/types/responses"
	"encoding/json"
	"strings"
)

const responsesToolUnavailableSystemMessage = "This compatibility backend cannot execute local tools, shell commands, web searches, or file operations. Do not claim to have run tools or inspected external resources. If a user asks you to use a tool, say that tool execution is unavailable through this backend."

func completionMessagesFromResponse(req *responses.ApiReq) []completions.ApiMessage {
	messages := make([]completions.ApiMessage, 0)
	if strings.TrimSpace(req.Instructions) != "" {
		messages = append(messages, completions.ApiMessage{Role: "system", Content: strings.TrimSpace(req.Instructions)})
	}
	if hasResponsesNonImageTools(req.Tools) {
		messages = append(messages, completions.ApiMessage{Role: "system", Content: responsesToolUnavailableSystemMessage})
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
		return []completions.ApiMessage{{Role: responseStringValue(v["role"], "user"), Content: responseMessageContentText(v)}}
	case []interface{}:
		messages := make([]completions.ApiMessage, 0, len(v))
		for _, item := range v {
			if part, ok := item.(map[string]interface{}); ok {
				text := responseMessageContentText(part)
				if strings.TrimSpace(text) != "" {
					messages = append(messages, completions.ApiMessage{Role: responseStringValue(part["role"], "user"), Content: text})
				}
			}
		}
		return messages
	default:
		return nil
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

func hasResponsesImageGenerationTool(tools []responses.Tool) bool {
	for _, tool := range tools {
		if strings.TrimSpace(tool.Type) == "image_generation" {
			return true
		}
	}
	return false
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
