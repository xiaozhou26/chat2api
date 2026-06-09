package completions

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseFunctionCallsXMLWithCDATAAndThink(t *testing.T) {
	content := `<think>` + ToolifyTriggerSignal + `</think>
prefix
` + ToolifyTriggerSignal + `
<function_calls>
  <function_call>
    <tool>search</tool>
    <args_json><![CDATA[{"query":"hello","limit":2}]]></args_json>
  </function_call>
</function_calls>`

	calls := ParseFunctionCallsXML(content, ToolifyTriggerSignal)
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Name != "search" {
		t.Fatalf("unexpected tool name: %s", calls[0].Name)
	}
	if calls[0].Args["query"] != "hello" {
		t.Fatalf("unexpected query arg: %#v", calls[0].Args["query"])
	}
	if calls[0].Args["limit"].(float64) != 2 {
		t.Fatalf("unexpected limit arg: %#v", calls[0].Args["limit"])
	}
}

func TestParseFunctionCallsXMLRejectsNonObjectArgsJSON(t *testing.T) {
	content := ToolifyTriggerSignal + `
<function_calls>
  <function_call>
    <tool>search</tool>
    <args_json>[1,2]</args_json>
  </function_call>
</function_calls>`

	if calls := ParseFunctionCallsXML(content, ToolifyTriggerSignal); len(calls) != 0 {
		t.Fatalf("expected no calls, got %#v", calls)
	}
}

func TestPreprocessMessagesConvertsToolResults(t *testing.T) {
	messages := []ApiMessage{
		{
			Role: "assistant",
			ToolCalls: []ToolCall{{
				ID:   "call_1",
				Type: "function",
				Function: ToolCallFunction{
					Name:      "search",
					Arguments: `{"query":"go"}`,
				},
			}},
		},
		{Role: "tool", ToolCallID: "call_1", Content: "result text"},
	}

	processed, err := PreprocessMessages(messages)
	if err != nil {
		t.Fatal(err)
	}
	if len(processed) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(processed))
	}
	if processed[0].ToolCalls != nil {
		t.Fatalf("assistant tool_calls should be converted to content")
	}
	if !strings.Contains(processed[0].Content.(string), "<function_calls>") {
		t.Fatalf("assistant content missing function_calls XML: %s", processed[0].Content)
	}
	if processed[1].Role != "user" {
		t.Fatalf("tool result should be converted to user role, got %s", processed[1].Role)
	}
	if !strings.Contains(processed[1].Content.(string), "Tool name: search") {
		t.Fatalf("tool result missing tool context: %s", processed[1].Content)
	}
}

func TestMessagesNeedPreprocessDetectsToolRoleWithoutTools(t *testing.T) {
	messages := []ApiMessage{
		{
			Role: "assistant",
			ToolCalls: []ToolCall{{
				ID:   "call_1",
				Type: "function",
				Function: ToolCallFunction{
					Name:      "search",
					Arguments: `{"query":"go"}`,
				},
			}},
		},
		{Role: "tool", ToolCallID: "call_1", Content: "result text"},
	}
	if !MessagesNeedPreprocess(messages) {
		t.Fatal("tool role should require preprocessing even when request has no tools")
	}
	processed, err := PreprocessMessages(messages)
	if err != nil {
		t.Fatal(err)
	}
	if processed[1].Role != "user" {
		t.Fatalf("tool role leaked through preprocessing: %s", processed[1].Role)
	}
	if !strings.Contains(processed[1].Content.(string), "Tool name: search") {
		t.Fatalf("missing tool result context: %s", processed[1].Content)
	}
}

func TestPreprocessMessagesRejectsUnknownToolCallIDLikeToolify(t *testing.T) {
	messages := []ApiMessage{{Role: "tool", ToolCallID: "call_1", Content: "result text"}}
	if _, err := PreprocessMessages(messages); err == nil {
		t.Fatal("expected unknown tool_call_id to be rejected")
	}
}

func TestBuildFunctionPromptValidatesToolChoice(t *testing.T) {
	tools := []Tool{{
		Type: "function",
		Function: ToolFunction{
			Name: "search",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{"type": "string"},
				},
				"required": []interface{}{"query"},
			},
		},
	}}
	_, err := BuildFunctionPrompt(tools, map[string]interface{}{
		"type":     "function",
		"function": map[string]interface{}{"name": "missing"},
	})
	if err == nil {
		t.Fatal("expected invalid tool_choice error")
	}
}

func TestStreamToolDetectorHoldsPartialTrigger(t *testing.T) {
	detector := NewStreamToolDetector(ToolifyTriggerSignal)
	detected, out := detector.ProcessChunk("hello " + ToolifyTriggerSignal[:10])
	if detected {
		t.Fatal("partial trigger should not detect")
	}
	if out != "hello " {
		t.Fatalf("unexpected output: %q", out)
	}
	detected, out = detector.ProcessChunk(ToolifyTriggerSignal[10:] + "\n<function_calls>")
	if !detected {
		t.Fatal("expected trigger detection")
	}
	if out != "" {
		t.Fatalf("trigger should not be emitted, got %q", out)
	}
	if detector.State() != "tool_parsing" {
		t.Fatalf("unexpected state: %s", detector.State())
	}
}

func TestToolCallFunctionAcceptsObjectArguments(t *testing.T) {
	var message ApiMessage
	err := json.Unmarshal([]byte(`{
		"role":"assistant",
		"tool_calls":[{
			"id":"call_1",
			"type":"function",
			"function":{
				"name":"search",
				"arguments":{"query":"go","limit":2}
			}
		}]
	}`), &message)
	if err != nil {
		t.Fatal(err)
	}
	args := message.ToolCalls[0].Function.Arguments
	if !strings.Contains(args, `"query":"go"`) || !strings.Contains(args, `"limit":2`) {
		t.Fatalf("arguments object was not normalized to JSON: %s", args)
	}
}

func TestToolCallsStreamChunkIncludesExplicitNullContent(t *testing.T) {
	chunk := NewToolCallsApiRespStream("chatcmpl_test", "auto", []ToolCall{{
		Index: intPtr(0),
		ID:    "call_1",
		Type:  "function",
		Function: ToolCallFunction{
			Name:      "search",
			Arguments: `{"query":"go"}`,
		},
	}})
	data := chunk.String()
	if !strings.Contains(data, `"content":null`) {
		t.Fatalf("tool call stream delta should include content:null, got %s", data)
	}
}

func TestStripFunctionCallXMLKeepsAnswerText(t *testing.T) {
	content := ToolifyTriggerSignal + `
<function_calls>
<function_call>
<tool>get_weather</tool>
<args_json><![CDATA[{"city":"呼和浩特"}]]></args_json>
</function_call>
</function_calls>呼和浩特现在是晴天，26℃。`

	stripped := StripFunctionCallXML(content)
	if strings.Contains(stripped, ToolifyTriggerSignal) || strings.Contains(stripped, "<function_calls>") {
		t.Fatalf("function call XML leaked after stripping: %s", stripped)
	}
	if stripped != "呼和浩特现在是晴天，26℃。" {
		t.Fatalf("unexpected stripped content: %s", stripped)
	}
}

func intPtr(value int) *int {
	return &value
}
