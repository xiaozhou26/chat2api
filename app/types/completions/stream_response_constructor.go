package completions

import "time"

func NewApiRespStream(id string, model string, content string) *ApiRespStream {
	return &ApiRespStream{
		ID:      id,
		Created: time.Now().Unix(),
		Object:  "chat.completion.chunk",
		Model:   model,
		Choices: []ApiStreamChoice{
			{
				Delta: ApiStreamDelta{
					Content: content,
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
