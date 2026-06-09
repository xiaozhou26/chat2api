package completions

import "time"

func NewApiRespStream(id string, model string, content string) *ApiRespStream {
	contentPtr := content
	return &ApiRespStream{
		ID:      id,
		Created: time.Now().Unix(),
		Object:  "chat.completion.chunk",
		Model:   model,
		Choices: []ApiStreamChoice{
			{
				Delta: ApiStreamDelta{
					Content: &contentPtr,
				},
				Index:        0,
				FinishReason: nil,
			},
		},
	}
}

func NewToolCallsApiRespStream(id string, model string, toolCalls []ToolCall) *ApiRespStream {
	return &ApiRespStream{
		ID:      id,
		Created: time.Now().Unix(),
		Object:  "chat.completion.chunk",
		Model:   model,
		Choices: []ApiStreamChoice{
			{
				Delta: ApiStreamDelta{
					Role:               "assistant",
					ToolCalls:          toolCalls,
					IncludeNullContent: true,
				},
				Index:        0,
				FinishReason: nil,
			},
		},
	}
}

func StopChunk(id string, model string, finishReason string) ApiRespStream {
	return ApiRespStream{
		ID:      id,
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []ApiStreamChoice{
			{
				Index:        0,
				FinishReason: finishReason,
			},
		},
	}
}
