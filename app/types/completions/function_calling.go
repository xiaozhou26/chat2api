package completions

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/google/uuid"
)

const ToolifyTriggerSignal = "<Function_chat2api_Start/>"

type ParsedToolCall struct {
	Name string
	Args map[string]interface{}
}

type StreamToolDetector struct {
	trigger      string
	buffer       string
	state        string
	thinkDepth   int
	signalLength int
}

func NewStreamToolDetector(trigger string) *StreamToolDetector {
	if trigger == "" {
		trigger = ToolifyTriggerSignal
	}
	return &StreamToolDetector{
		trigger:      trigger,
		state:        "detecting",
		signalLength: len(trigger),
	}
}

func (d *StreamToolDetector) State() string {
	return d.state
}

func (d *StreamToolDetector) Buffer() string {
	return d.buffer
}

func (d *StreamToolDetector) AppendParsing(content string) {
	d.buffer += content
}

func (d *StreamToolDetector) HasCompleteToolBlock() bool {
	return strings.Contains(d.buffer, "</function_calls>")
}

func (d *StreamToolDetector) ProcessChunk(delta string) (bool, string) {
	if delta == "" {
		return false, ""
	}
	d.buffer += delta
	if d.state == "tool_parsing" {
		return false, ""
	}

	var out strings.Builder
	i := 0
	for i < len(d.buffer) {
		if strings.HasPrefix(d.buffer[i:], "<think>") {
			d.thinkDepth++
			out.WriteString("<think>")
			i += len("<think>")
			continue
		}
		if strings.HasPrefix(d.buffer[i:], "</think>") {
			if d.thinkDepth > 0 {
				d.thinkDepth--
			}
			out.WriteString("</think>")
			i += len("</think>")
			continue
		}
		if d.thinkDepth == 0 && strings.HasPrefix(d.buffer[i:], d.trigger) {
			d.state = "tool_parsing"
			d.buffer = d.buffer[i:]
			return true, out.String()
		}
		if d.shouldHoldSuffix(d.buffer[i:]) {
			break
		}
		out.WriteByte(d.buffer[i])
		i++
	}
	d.buffer = d.buffer[i:]
	return false, out.String()
}

func (d *StreamToolDetector) shouldHoldSuffix(suffix string) bool {
	if suffix == "" {
		return false
	}
	for _, marker := range []string{d.trigger, "<think>", "</think>"} {
		if len(suffix) < len(marker) && strings.HasPrefix(marker, suffix) {
			return true
		}
	}
	return false
}

func (d *StreamToolDetector) FlushText() string {
	if d.state != "detecting" {
		return ""
	}
	text := d.buffer
	d.buffer = ""
	return text
}

func (d *StreamToolDetector) Finalize() []ParsedToolCall {
	if d.state != "tool_parsing" {
		return nil
	}
	return ParseFunctionCallsXML(d.buffer, d.trigger)
}

func NormalizeLegacyFunctions(apiReq *ApiReq) {
	if len(apiReq.Tools) == 0 && len(apiReq.Functions) > 0 {
		apiReq.Tools = make([]Tool, 0, len(apiReq.Functions))
		for _, function := range apiReq.Functions {
			apiReq.Tools = append(apiReq.Tools, Tool{Type: "function", Function: function})
		}
	}
	if apiReq.ToolChoice == nil && apiReq.FunctionCall != nil {
		apiReq.ToolChoice = legacyFunctionCallToToolChoice(apiReq.FunctionCall)
	}
}

func HasTools(apiReq *ApiReq) bool {
	NormalizeLegacyFunctions(apiReq)
	for _, tool := range apiReq.Tools {
		if strings.TrimSpace(tool.Function.Name) != "" {
			return true
		}
	}
	return false
}

func MessagesNeedPreprocess(messages []ApiMessage) bool {
	for _, message := range messages {
		switch strings.TrimSpace(message.Role) {
		case "tool", "function", "developer":
			return true
		}
		if len(message.ToolCalls) > 0 || message.FunctionCall != nil {
			return true
		}
	}
	return false
}

func MessagesContainToolResults(messages []ApiMessage) bool {
	for _, message := range messages {
		role := strings.TrimSpace(message.Role)
		if role == "tool" || role == "function" {
			return true
		}
		content := contentToText(message.Content)
		if strings.Contains(content, "<tool_result>") && strings.Contains(content, "</tool_result>") {
			return true
		}
	}
	return false
}

func BuildFunctionPrompt(tools []Tool, toolChoice interface{}) (string, error) {
	toolList, err := buildToolsList(tools)
	if err != nil {
		return "", err
	}
	prompt := functionPromptTemplate(ToolifyTriggerSignal, toolList)
	choicePrompt, err := toolChoicePrompt(toolChoice, tools)
	if err != nil {
		return "", err
	}
	if choicePrompt != "" {
		prompt += choicePrompt
	}
	return prompt, nil
}

func PreprocessMessages(messages []ApiMessage) ([]ApiMessage, error) {
	index := buildToolCallIndex(messages)
	processed := make([]ApiMessage, 0, len(messages))
	for _, message := range messages {
		role := strings.TrimSpace(message.Role)
		switch role {
		case "tool":
			if strings.TrimSpace(message.ToolCallID) == "" {
				return nil, fmt.Errorf("tool message missing tool_call_id")
			}
			if message.Content == nil {
				return nil, fmt.Errorf("tool message missing content for tool_call_id=%s", message.ToolCallID)
			}
			toolInfo, ok := index[message.ToolCallID]
			if !ok {
				return nil, fmt.Errorf("tool_call_id=%s not found in conversation history. Ensure the assistant message with this tool_call is included in the messages array", message.ToolCallID)
			}
			processed = append(processed, ApiMessage{
				Role:    "user",
				Content: formatToolResultForAI(toolInfo.Name, toolInfo.Arguments, contentToText(message.Content)),
			})
		case "assistant":
			if len(message.ToolCalls) == 0 && message.FunctionCall != nil {
				message.ToolCalls = []ToolCall{{
					ID:   "call_legacy_function",
					Type: "function",
					Function: ToolCallFunction{
						Name:      message.FunctionCall.Name,
						Arguments: message.FunctionCall.Arguments,
					},
				}}
			}
			if len(message.ToolCalls) > 0 {
				formatted, err := FormatAssistantToolCallsForAI(message.ToolCalls)
				if err != nil {
					return nil, err
				}
				content := strings.TrimSpace(contentToText(message.Content))
				if content != "" {
					content += "\n"
				}
				message.Content = strings.TrimSpace(content + formatted)
				message.ToolCalls = nil
				message.FunctionCall = nil
			}
			processed = append(processed, message)
		case "developer":
			message.Role = "system"
			processed = append(processed, message)
		case "function":
			processed = append(processed, ApiMessage{
				Role: "user",
				Content: fmt.Sprintf("Function execution result:\n- Function name: %s\n- Execution result:\n<tool_result>\n%s\n</tool_result>",
					message.Name,
					contentToText(message.Content),
				),
			})
		default:
			processed = append(processed, message)
		}
	}
	return processed, nil
}

func ParseFunctionCallsXML(content string, trigger string) []ParsedToolCall {
	if trigger == "" {
		trigger = ToolifyTriggerSignal
	}
	if content == "" || !strings.Contains(content, trigger) {
		return nil
	}
	cleaned := removeThinkBlocks(content)
	signalPositions := allStringPositions(cleaned, trigger)
	for i := len(signalPositions) - 1; i >= 0; i-- {
		sub := cleaned[signalPositions[i]:]
		callsXML, callsContent, ok := extractFunctionCallsBlock(sub)
		if !ok {
			continue
		}
		if parsed := parseFunctionCallsStrict(callsXML); len(parsed) > 0 {
			return parsed
		}
		if parsed := parseFunctionCallsRegex(callsContent); len(parsed) > 0 {
			return parsed
		}
		return nil
	}
	return nil
}

func FindLastTriggerOutsideThink(text string, trigger string) int {
	if trigger == "" {
		trigger = ToolifyTriggerSignal
	}
	if text == "" || trigger == "" {
		return -1
	}
	depth := 0
	last := -1
	for i := 0; i < len(text); {
		switch {
		case strings.HasPrefix(text[i:], "<think>"):
			depth++
			i += len("<think>")
		case strings.HasPrefix(text[i:], "</think>"):
			if depth > 0 {
				depth--
			}
			i += len("</think>")
		case depth == 0 && strings.HasPrefix(text[i:], trigger):
			last = i
			i++
		default:
			i++
		}
	}
	return last
}

func ToolCallsFromParsed(parsed []ParsedToolCall, includeIndex bool) []ToolCall {
	toolCalls := make([]ToolCall, 0, len(parsed))
	for i, call := range parsed {
		args, _ := json.Marshal(call.Args)
		toolCall := ToolCall{
			ID:   "call_" + strings.ReplaceAll(uuid.New().String(), "-", ""),
			Type: "function",
			Function: ToolCallFunction{
				Name:      call.Name,
				Arguments: string(args),
			},
		}
		if includeIndex {
			idx := i
			toolCall.Index = &idx
		}
		toolCalls = append(toolCalls, toolCall)
	}
	return toolCalls
}

func ValidateParsedToolCalls(parsed []ParsedToolCall, tools []Tool) error {
	allowed := make(map[string]map[string]interface{})
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		name := strings.TrimSpace(tool.Function.Name)
		if name == "" {
			continue
		}
		allowed[name] = tool.Function.Parameters
		names = append(names, name)
	}
	sort.Strings(names)
	for i, call := range parsed {
		if strings.TrimSpace(call.Name) == "" {
			return fmt.Errorf("tool call #%d: missing tool name", i+1)
		}
		schema, ok := allowed[call.Name]
		if !ok {
			return fmt.Errorf("tool call #%d: unknown tool %q, allowed tools: %v", i+1, call.Name, names)
		}
		if call.Args == nil {
			return fmt.Errorf("tool call #%d %q: arguments must be a JSON object", i+1, call.Name)
		}
		if err := validateObjectAgainstSchema(call.Args, schema, call.Name, 0); err != nil {
			return fmt.Errorf("tool call #%d %q: %w", i+1, call.Name, err)
		}
	}
	return nil
}

func ToolCallPrefixText(content string) string {
	pos := FindLastTriggerOutsideThink(content, ToolifyTriggerSignal)
	if pos == -1 {
		return content
	}
	return strings.TrimSpace(content[:pos])
}

func StripFunctionCallXML(content string) string {
	pos := FindLastTriggerOutsideThink(content, ToolifyTriggerSignal)
	if pos == -1 {
		return content
	}
	afterTrigger := content[pos:]
	match := functionCallsRE.FindStringIndex(afterTrigger)
	if len(match) != 2 {
		return content
	}
	if strings.TrimSpace(afterTrigger[:match[0]]) != ToolifyTriggerSignal {
		return content
	}
	before := strings.TrimSpace(content[:pos])
	after := strings.TrimSpace(afterTrigger[match[1]:])
	switch {
	case before == "":
		return after
	case after == "":
		return before
	default:
		return before + "\n\n" + after
	}
}

func legacyFunctionCallToToolChoice(value interface{}) interface{} {
	switch v := value.(type) {
	case string:
		switch strings.TrimSpace(v) {
		case "", "auto":
			return "auto"
		case "none":
			return "none"
		default:
			return v
		}
	case map[string]interface{}:
		if name, ok := v["name"].(string); ok && strings.TrimSpace(name) != "" {
			return map[string]interface{}{"type": "function", "function": map[string]interface{}{"name": strings.TrimSpace(name)}}
		}
	}
	return value
}

func buildToolsList(tools []Tool) (string, error) {
	parts := make([]string, 0, len(tools))
	for i, tool := range tools {
		if tool.Type == "" {
			tool.Type = "function"
		}
		if tool.Type != "function" {
			continue
		}
		name := strings.TrimSpace(tool.Function.Name)
		if name == "" {
			return "", fmt.Errorf("tool #%d: function.name is required", i+1)
		}
		schema := tool.Function.Parameters
		if schema == nil {
			schema = map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
		}
		props, err := objectMap(schema["properties"])
		if err != nil {
			return "", fmt.Errorf("tool %q: properties must be an object", name)
		}
		required, err := stringSlice(schema["required"])
		if err != nil {
			return "", fmt.Errorf("tool %q: required must be a list of strings", name)
		}
		missing := missingRequiredProperties(required, props)
		if len(missing) > 0 {
			return "", fmt.Errorf("tool %q: required parameters %v are not defined in properties", name, missing)
		}
		summary := parameterSummary(props)
		if summary == "" {
			summary = "None"
		}
		details := parameterDetails(props, required)
		if details == "" {
			details = "(no parameter details)"
		}
		description := strings.TrimSpace(tool.Function.Description)
		descBlock := "None"
		if description != "" {
			descBlock = "```\n" + description + "\n```"
		}
		requiredText := "None"
		if len(required) > 0 {
			requiredText = strings.Join(required, ", ")
		}
		parts = append(parts, fmt.Sprintf("%d. <tool name=\"%s\">\n   Description:\n%s\n   Parameters summary: %s\n   Required parameters: %s\n   Parameter details:\n%s",
			i+1, name, descBlock, summary, requiredText, details))
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("tools must include at least one function tool")
	}
	return strings.Join(parts, "\n\n"), nil
}

func functionPromptTemplate(trigger string, toolsList string) string {
	return fmt.Sprintf(`
You have access to the following available tools to help solve problems:

%s

IMPORTANT CONTEXT NOTES:
1. You can call MULTIPLE tools in a single response if needed.
2. Respect the user's latest constraints and preferences, including requests to use no tools, one tool, or a specific tool.
3. The conversation may already contain tool execution results from previous function calls. Review the history to avoid duplicate tool calls.
4. Tool execution results are formatted with <tool_result>...</tool_result> tags.
5. This is the ONLY format you can use for tool calls.

When you need to use tools, you MUST strictly follow this format:

1. Begin on a new line with exactly:
%s
No leading or trailing spaces. The trigger signal MUST be on its own line and appear only once.

2. Immediately follow with one complete <function_calls> XML block.

3. For multiple tool calls, include multiple <function_call> blocks inside the same <function_calls> wrapper.

4. Do not add any text after the closing </function_calls> tag.

STRICT ARGUMENT KEY RULES:
- Use parameter keys EXACTLY as defined, including case and punctuation.
- If a key starts with a hyphen, keep the leading hyphen in the JSON key.
- The <tool> tag must contain the exact name of a declared tool.
- The <args_json> tag must contain a single JSON object.
- You MAY wrap JSON inside <![CDATA[...]]> to avoid XML escaping issues.

CORRECT Example:
%s
<function_calls>
    <function_call>
        <tool>search</tool>
        <args_json><![CDATA[{"keywords":["Python Document","how to use python"]}]]></args_json>
    </function_call>
</function_calls>

Now please be ready to strictly follow the above specifications.
`, toolsList, trigger, trigger)
}

func toolChoicePrompt(toolChoice interface{}, tools []Tool) (string, error) {
	switch v := toolChoice.(type) {
	case nil:
		return "", nil
	case string:
		switch strings.TrimSpace(v) {
		case "", "auto":
			return "", nil
		case "none":
			return "\n\nIMPORTANT: You are prohibited from using any tools in this round. Respond normally and directly.", nil
		case "required":
			return "\n\nIMPORTANT: You MUST call at least one tool in this response. Do not respond without using tools.", nil
		default:
			return "", nil
		}
	case map[string]interface{}:
		if strings.TrimSpace(stringFromMap(v, "type")) != "function" {
			return "", nil
		}
		function, _ := v["function"].(map[string]interface{})
		name := strings.TrimSpace(stringFromMap(function, "name"))
		if name == "" {
			return "", fmt.Errorf("tool_choice.function.name must be a non-empty string")
		}
		if !toolNameExists(name, tools) {
			return "", fmt.Errorf("tool_choice specifies tool %q which is not in the tools list", name)
		}
		return fmt.Sprintf("\n\nIMPORTANT: In this round, you must use ONLY the tool named `%s`. Generate the necessary parameters and output in the specified XML format.", name), nil
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return "", nil
		}
		var obj map[string]interface{}
		if err := json.Unmarshal(data, &obj); err != nil {
			return "", nil
		}
		return toolChoicePrompt(obj, tools)
	}
}

func toolNameExists(name string, tools []Tool) bool {
	for _, tool := range tools {
		if tool.Function.Name == name {
			return true
		}
	}
	return false
}

type toolCallInfo struct {
	Name      string
	Arguments string
}

func buildToolCallIndex(messages []ApiMessage) map[string]toolCallInfo {
	index := make(map[string]toolCallInfo)
	for _, message := range messages {
		if message.Role != "assistant" {
			continue
		}
		toolCalls := message.ToolCalls
		if len(toolCalls) == 0 && message.FunctionCall != nil {
			toolCalls = []ToolCall{{
				ID:   "call_legacy_function",
				Type: "function",
				Function: ToolCallFunction{
					Name:      message.FunctionCall.Name,
					Arguments: message.FunctionCall.Arguments,
				},
			}}
		}
		for _, call := range toolCalls {
			if strings.TrimSpace(call.ID) == "" || strings.TrimSpace(call.Function.Name) == "" {
				continue
			}
			args := strings.TrimSpace(call.Function.Arguments)
			if args == "" {
				args = "{}"
			}
			index[call.ID] = toolCallInfo{Name: call.Function.Name, Arguments: args}
		}
	}
	return index
}

func formatToolResultForAI(toolName string, toolArguments string, resultContent string) string {
	return fmt.Sprintf("Tool execution result:\n- Tool name: %s\n- Tool arguments: %s\n- Execution result:\n<tool_result>\n%s\n</tool_result>",
		toolName,
		toolArguments,
		resultContent,
	)
}

func FormatAssistantToolCallsForAI(toolCalls []ToolCall) (string, error) {
	blocks := make([]string, 0, len(toolCalls))
	for _, call := range toolCalls {
		args := strings.TrimSpace(call.Function.Arguments)
		if args == "" {
			args = "{}"
		}
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(args), &obj); err != nil {
			return "", fmt.Errorf("invalid assistant.tool_calls arguments for tool %q: %w", call.Function.Name, err)
		}
		normalizedArgs, _ := json.Marshal(obj)
		blocks = append(blocks, fmt.Sprintf("<function_call>\n<tool>%s</tool>\n<args_json><![CDATA[%s]]></args_json>\n</function_call>",
			call.Function.Name,
			escapeCDATA(string(normalizedArgs)),
		))
	}
	return fmt.Sprintf("%s\n<function_calls>\n%s\n</function_calls>", ToolifyTriggerSignal, strings.Join(blocks, "\n")), nil
}

func contentToText(content interface{}) string {
	switch v := content.(type) {
	case nil:
		return ""
	case string:
		return v
	case []interface{}:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			parts = append(parts, contentToText(item))
		}
		return strings.Join(parts, "")
	case map[string]interface{}:
		if text, ok := v["text"].(string); ok {
			return text
		}
		if text, ok := v["content"].(string); ok {
			return text
		}
		data, _ := json.Marshal(v)
		return string(data)
	default:
		data, _ := json.Marshal(v)
		return string(data)
	}
}

func removeThinkBlocks(text string) string {
	for {
		start := strings.Index(text, "<think>")
		if start == -1 {
			return text
		}
		pos := start + len("<think>")
		depth := 1
		for pos < len(text) && depth > 0 {
			switch {
			case strings.HasPrefix(text[pos:], "<think>"):
				depth++
				pos += len("<think>")
			case strings.HasPrefix(text[pos:], "</think>"):
				depth--
				pos += len("</think>")
			default:
				pos++
			}
		}
		if depth != 0 {
			return text
		}
		text = text[:start] + text[pos:]
	}
}

func allStringPositions(text string, needle string) []int {
	positions := make([]int, 0)
	for start := 0; ; {
		pos := strings.Index(text[start:], needle)
		if pos == -1 {
			return positions
		}
		absolute := start + pos
		positions = append(positions, absolute)
		start = absolute + 1
	}
}

var functionCallsRE = regexp.MustCompile(`(?s)<function_calls>(.*?)</function_calls>`)

func extractFunctionCallsBlock(text string) (string, string, bool) {
	match := functionCallsRE.FindStringSubmatch(text)
	if len(match) != 2 {
		return "", "", false
	}
	return match[0], match[1], true
}

type xmlFunctionCalls struct {
	Calls []xmlFunctionCall `xml:"function_call"`
}

type xmlFunctionCall struct {
	Tool     string `xml:"tool"`
	ArgsJSON string `xml:"args_json"`
	Args     []struct {
		XMLName xml.Name
		Value   string `xml:",chardata"`
	} `xml:"args>*"`
}

func parseFunctionCallsStrict(callsXML string) []ParsedToolCall {
	var root xmlFunctionCalls
	if err := xml.Unmarshal([]byte(callsXML), &root); err != nil {
		return nil
	}
	results := make([]ParsedToolCall, 0, len(root.Calls))
	for _, call := range root.Calls {
		name := strings.TrimSpace(call.Tool)
		if name == "" {
			continue
		}
		args := map[string]interface{}{}
		if strings.TrimSpace(call.ArgsJSON) != "" {
			parsed, ok := parseArgsJSONPayload(call.ArgsJSON)
			if !ok {
				return nil
			}
			args = parsed
		} else {
			for _, arg := range call.Args {
				args[arg.XMLName.Local] = coerceXMLArg(arg.Value)
			}
		}
		results = append(results, ParsedToolCall{Name: name, Args: args})
	}
	return results
}

var (
	functionCallBlockRE = regexp.MustCompile(`(?s)<function_call>(.*?)</function_call>`)
	toolTagRE           = regexp.MustCompile(`(?s)<tool>(.*?)</tool>`)
	argsJSONTagRE       = regexp.MustCompile(`(?s)<args_json>(.*?)</args_json>`)
	argsBlockRE         = regexp.MustCompile(`(?s)<args>(.*?)</args>`)
	argTagRE            = regexp.MustCompile(`(?s)<([^\s>/]+)>(.*?)</([^\s>/]+)>`)
	cdataRE             = regexp.MustCompile(`(?s)<!\[CDATA\[(.*?)\]\]>`)
	codeFenceStartRE    = regexp.MustCompile(`(?s)^` + "```" + `(?:json)?\s*`)
	codeFenceEndRE      = regexp.MustCompile(`(?s)\s*` + "```" + `$`)
)

func parseFunctionCallsRegex(callsContent string) []ParsedToolCall {
	blocks := functionCallBlockRE.FindAllStringSubmatch(callsContent, -1)
	results := make([]ParsedToolCall, 0, len(blocks))
	for _, block := range blocks {
		body := block[1]
		toolMatch := toolTagRE.FindStringSubmatch(body)
		if len(toolMatch) != 2 {
			continue
		}
		name := strings.TrimSpace(toolMatch[1])
		if name == "" {
			continue
		}
		args := map[string]interface{}{}
		if argsMatch := argsJSONTagRE.FindStringSubmatch(body); len(argsMatch) == 2 {
			payload := extractCDATAText(argsMatch[1])
			parsed, ok := parseArgsJSONPayload(payload)
			if !ok {
				return nil
			}
			args = parsed
		} else if argsBlock := argsBlockRE.FindStringSubmatch(body); len(argsBlock) == 2 {
			for _, argMatch := range argTagRE.FindAllStringSubmatch(argsBlock[1], -1) {
				if len(argMatch) == 4 && argMatch[1] == argMatch[3] {
					args[argMatch[1]] = coerceXMLArg(argMatch[2])
				}
			}
		}
		results = append(results, ParsedToolCall{Name: name, Args: args})
	}
	return results
}

func parseArgsJSONPayload(payload string) (map[string]interface{}, bool) {
	s := strings.TrimSpace(extractCDATAText(payload))
	if s == "" {
		return map[string]interface{}{}, true
	}
	if strings.HasPrefix(s, "```") {
		s = codeFenceStartRE.ReplaceAllString(s, "")
		s = codeFenceEndRE.ReplaceAllString(s, "")
	}
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(s), &obj); err != nil {
		return nil, false
	}
	if obj == nil {
		obj = map[string]interface{}{}
	}
	return obj, true
}

func extractCDATAText(raw string) string {
	if !strings.Contains(raw, "<![CDATA[") {
		return raw
	}
	matches := cdataRE.FindAllStringSubmatch(raw, -1)
	if len(matches) == 0 {
		return raw
	}
	var b strings.Builder
	for _, match := range matches {
		if len(match) == 2 {
			b.WriteString(match[1])
		}
	}
	return b.String()
}

func coerceXMLArg(value string) interface{} {
	value = strings.TrimSpace(value)
	var parsed interface{}
	if err := json.Unmarshal([]byte(value), &parsed); err == nil {
		return parsed
	}
	return value
}

func escapeCDATA(text string) string {
	return strings.ReplaceAll(text, "]]>", "]]]]><![CDATA[>")
}

func objectMap(value interface{}) (map[string]interface{}, error) {
	if value == nil {
		return map[string]interface{}{}, nil
	}
	if obj, ok := value.(map[string]interface{}); ok {
		return obj, nil
	}
	return nil, fmt.Errorf("not an object")
}

func stringSlice(value interface{}) ([]string, error) {
	if value == nil {
		return nil, nil
	}
	list, ok := value.([]interface{})
	if !ok {
		if stringsList, ok := value.([]string); ok {
			return stringsList, nil
		}
		return nil, fmt.Errorf("not a list")
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		text, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("not a string")
		}
		out = append(out, text)
	}
	return out, nil
}

func missingRequiredProperties(required []string, props map[string]interface{}) []string {
	missing := make([]string, 0)
	for _, key := range required {
		if _, ok := props[key]; !ok {
			missing = append(missing, key)
		}
	}
	return missing
}

func parameterSummary(props map[string]interface{}) string {
	keys := sortedKeys(props)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		schema, _ := props[key].(map[string]interface{})
		parts = append(parts, fmt.Sprintf("%s (%s)", key, schemaTypeName(schema)))
	}
	return strings.Join(parts, ", ")
}

func parameterDetails(props map[string]interface{}, required []string) string {
	requiredSet := make(map[string]bool)
	for _, key := range required {
		requiredSet[key] = true
	}
	keys := sortedKeys(props)
	lines := make([]string, 0)
	for _, key := range keys {
		lines = append(lines, "- "+key+":")
		appendSchemaBody(&lines, props[key], requiredSet[key], 1, 0)
	}
	return strings.Join(lines, "\n")
}

func appendSchemaBody(lines *[]string, schemaValue interface{}, isRequired bool, indentLevel int, depth int) {
	indent := strings.Repeat("  ", indentLevel)
	schema, _ := schemaValue.(map[string]interface{})
	if schema == nil {
		schema = map[string]interface{}{}
	}
	if depth > 8 {
		*lines = append(*lines, indent+"- note: nested schema omitted after depth 8")
		return
	}
	*lines = append(*lines, indent+"- type: "+schemaTypeName(schema))
	*lines = append(*lines, fmt.Sprintf("%s- required: %s", indent, yesNo(isRequired)))
	if description := strings.TrimSpace(stringValue(schema["description"])); description != "" {
		*lines = append(*lines, indent+"- description: "+description)
	}
	if enumValue, ok := schema["enum"]; ok {
		*lines = append(*lines, indent+"- enum: "+jsonString(enumValue))
	}
	if constValue, ok := schema["const"]; ok {
		*lines = append(*lines, indent+"- const: "+jsonString(constValue))
	}
	if defaultValue, ok := schema["default"]; ok {
		*lines = append(*lines, indent+"- default: "+jsonString(defaultValue))
	}
	constraints := collectSchemaConstraints(schema)
	if len(constraints) > 0 {
		*lines = append(*lines, indent+"- constraints: "+jsonString(constraints))
	}
	props, _ := objectMap(schema["properties"])
	required, _ := stringSlice(schema["required"])
	if len(required) > 0 {
		*lines = append(*lines, indent+"- required properties: "+strings.Join(required, ", "))
	}
	if len(props) > 0 {
		requiredSet := make(map[string]bool)
		for _, key := range required {
			requiredSet[key] = true
		}
		*lines = append(*lines, indent+"- properties:")
		for _, childName := range sortedKeys(props) {
			childIndent := strings.Repeat("  ", indentLevel+1)
			*lines = append(*lines, childIndent+"- "+childName+":")
			appendSchemaBody(lines, props[childName], requiredSet[childName], indentLevel+2, depth+1)
		}
	}
	if items, ok := schema["items"].(map[string]interface{}); ok {
		*lines = append(*lines, indent+"- items:")
		appendSchemaBody(lines, items, false, indentLevel+1, depth+1)
	}
	if additional, ok := schema["additionalProperties"]; ok {
		switch v := additional.(type) {
		case bool:
			if !v {
				*lines = append(*lines, indent+"- additionalProperties: false")
			}
		case map[string]interface{}:
			*lines = append(*lines, indent+"- additionalProperties:")
			appendSchemaBody(lines, v, false, indentLevel+1, depth+1)
		}
	}
	for _, keyword := range []string{"anyOf", "oneOf", "allOf"} {
		options, ok := schema[keyword].([]interface{})
		if !ok || len(options) == 0 {
			continue
		}
		*lines = append(*lines, indent+"- "+keyword+":")
		for i, option := range options {
			optionIndent := strings.Repeat("  ", indentLevel+1)
			*lines = append(*lines, fmt.Sprintf("%s- option %d:", optionIndent, i+1))
			appendSchemaBody(lines, option, false, indentLevel+2, depth+1)
		}
	}
}

func schemaTypeName(schema map[string]interface{}) string {
	if schema == nil {
		return "any"
	}
	switch v := schema["type"].(type) {
	case string:
		if strings.TrimSpace(v) != "" {
			return v
		}
	case []interface{}:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			if text, ok := item.(string); ok {
				parts = append(parts, text)
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, " | ")
		}
	case []string:
		if len(v) > 0 {
			return strings.Join(v, " | ")
		}
	}
	if _, ok := schema["properties"]; ok {
		return "object"
	}
	if _, ok := schema["items"]; ok {
		return "array"
	}
	for _, key := range []string{"anyOf", "oneOf", "allOf"} {
		if _, ok := schema[key]; ok {
			return key
		}
	}
	return "any"
}

func collectSchemaConstraints(schema map[string]interface{}) map[string]interface{} {
	keys := []string{
		"minimum", "maximum", "exclusiveMinimum", "exclusiveMaximum",
		"minLength", "maxLength", "pattern", "format",
		"minItems", "maxItems", "uniqueItems",
		"minProperties", "maxProperties", "multipleOf",
	}
	out := make(map[string]interface{})
	for _, key := range keys {
		if value, ok := schema[key]; ok {
			out[key] = value
		}
	}
	return out
}

func validateObjectAgainstSchema(value map[string]interface{}, schema map[string]interface{}, path string, depth int) error {
	if schema == nil || depth > 8 {
		return nil
	}
	required, err := stringSlice(schema["required"])
	if err != nil {
		return nil
	}
	for _, key := range required {
		if _, ok := value[key]; !ok {
			return fmt.Errorf("%s: missing required property %q", path, key)
		}
	}
	props, _ := objectMap(schema["properties"])
	if additional, ok := schema["additionalProperties"].(bool); ok && !additional {
		for key := range value {
			if _, ok := props[key]; !ok {
				return fmt.Errorf("%s: unexpected property %q", path, key)
			}
		}
	}
	for key, raw := range value {
		propSchema, ok := props[key].(map[string]interface{})
		if !ok {
			continue
		}
		if err := validateValueAgainstSchema(raw, propSchema, path+"."+key, depth+1); err != nil {
			return err
		}
	}
	return nil
}

func validateValueAgainstSchema(value interface{}, schema map[string]interface{}, path string, depth int) error {
	if schema == nil || depth > 8 {
		return nil
	}
	if enumValues, ok := schema["enum"].([]interface{}); ok {
		found := false
		for _, enumValue := range enumValues {
			if fmt.Sprint(enumValue) == fmt.Sprint(value) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("%s: expected one of %s", path, jsonString(enumValues))
		}
	}
	typeName := schemaTypeName(schema)
	if typeName != "any" && !strings.Contains(typeName, " | ") && !schemaValueTypeOK(value, typeName) {
		return fmt.Errorf("%s: expected type %q, got %s", path, typeName, schemaTypeOfValue(value))
	}
	switch typed := value.(type) {
	case map[string]interface{}:
		return validateObjectAgainstSchema(typed, schema, path, depth+1)
	case []interface{}:
		itemSchema, _ := schema["items"].(map[string]interface{})
		for i, item := range typed {
			if err := validateValueAgainstSchema(item, itemSchema, fmt.Sprintf("%s[%d]", path, i), depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}

func schemaValueTypeOK(value interface{}, typeName string) bool {
	switch typeName {
	case "object":
		_, ok := value.(map[string]interface{})
		return ok
	case "array":
		_, ok := value.([]interface{})
		return ok
	case "string":
		_, ok := value.(string)
		return ok
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "integer":
		switch value.(type) {
		case int, int64, int32:
			return true
		case float64:
			return value.(float64) == float64(int64(value.(float64)))
		default:
			return false
		}
	case "number":
		switch value.(type) {
		case int, int64, int32, float64, float32:
			return true
		default:
			return false
		}
	case "null":
		return value == nil
	default:
		return true
	}
}

func schemaTypeOfValue(value interface{}) string {
	switch value.(type) {
	case nil:
		return "null"
	case map[string]interface{}:
		return "object"
	case []interface{}:
		return "array"
	case string:
		return "string"
	case bool:
		return "boolean"
	case float64, float32, int, int64, int32:
		return "number"
	default:
		return fmt.Sprintf("%T", value)
	}
}

func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func yesNo(value bool) string {
	if value {
		return "Yes"
	}
	return "No"
}

func jsonString(value interface{}) string {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprint(value)
	}
	return string(data)
}

func stringFromMap(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	return stringValue(m[key])
}
