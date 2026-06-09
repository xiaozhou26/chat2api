package completions

import "encoding/json"

type ToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	Strict      *bool                  `json:"strict,omitempty"`
}

type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolChoice struct {
	Type     string `json:"type"`
	Function struct {
		Name string `json:"name"`
	} `json:"function"`
}

type ToolCallFunction struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

func (f *ToolCallFunction) UnmarshalJSON(data []byte) error {
	var raw struct {
		Name      string      `json:"name"`
		Arguments interface{} `json:"arguments"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	f.Name = raw.Name
	f.Arguments = normalizeFunctionArguments(raw.Arguments)
	return nil
}

type ToolCall struct {
	Index    *int             `json:"index,omitempty"`
	ID       string           `json:"id,omitempty"`
	Type     string           `json:"type,omitempty"`
	Function ToolCallFunction `json:"function,omitempty"`
}

type FunctionCall struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

func (f *FunctionCall) UnmarshalJSON(data []byte) error {
	var raw struct {
		Name      string      `json:"name"`
		Arguments interface{} `json:"arguments"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	f.Name = raw.Name
	f.Arguments = normalizeFunctionArguments(raw.Arguments)
	return nil
}

func normalizeFunctionArguments(value interface{}) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return ""
		}
		return string(data)
	}
}

type ApiReq struct {
	Messages        []ApiMessage   `json:"messages"`
	Model           string         `json:"model"`
	Stream          bool           `json:"stream"`
	Tools           []Tool         `json:"tools,omitempty"`
	ToolChoice      interface{}    `json:"tool_choice,omitempty"`
	Functions       []ToolFunction `json:"functions,omitempty"`
	FunctionCall    interface{}    `json:"function_call,omitempty"`
	PluginIds       []string       `json:"plugin_ids"`
	ConversationId  string         `json:"conversation_id"`
	ParentMessageId string         `json:"parent_message_id"`
	NewMessages     string         `json:"-"`
	HasToolResults  bool           `json:"-"`
}

type ApiMessage struct {
	Role         string        `json:"role"`
	Content      interface{}   `json:"content,omitempty"`
	Name         string        `json:"name,omitempty"`
	ToolCalls    []ToolCall    `json:"tool_calls,omitempty"`
	ToolCallID   string        `json:"tool_call_id,omitempty"`
	FunctionCall *FunctionCall `json:"function_call,omitempty"`
}
