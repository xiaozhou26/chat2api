use serde::{Deserialize, Serialize};

// ===== Chat Request Types =====

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatAuthor {
    pub role: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatContent {
    pub content_type: String,
    pub parts: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatMessages {
    pub id: String,
    pub author: ChatAuthor,
    pub content: ChatContent,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatConversationMode {
    pub kind: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ClientContextualInfo {
    pub is_dark_mode: bool,
    pub time_since_loaded: i32,
    pub page_height: i32,
    pub page_width: i32,
    pub pixel_ratio: f64,
    pub screen_height: i32,
    pub screen_width: i32,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatReq {
    pub action: String,
    pub messages: Vec<ChatMessages>,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub conversation_id: String,
    pub parent_message_id: String,
    pub model: String,
    pub timezone: String,
    pub timezone_offset_min: i32,
    pub suggestions: Vec<String>,
    pub supported_encodings: Vec<String>,
    pub system_hints: Vec<String>,
    pub history_and_training_disabled: bool,
    pub force_use_sse: bool,
    pub face_use_sse: bool,
    pub force_paragen: bool,
    pub force_paragen_model_slug: String,
    pub force_rate_limit: bool,
    pub reset_rate_limits: bool,
    pub variant_purpose: String,
    pub conversation_mode: ChatConversationMode,
    pub websocket_request_id: String,
    pub client_contextual_info: ClientContextualInfo,
}

// ===== Chat Response Types =====

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatResp {
    pub message: ChatRespMessage,
    #[serde(default)]
    pub conversation_id: String,
    #[serde(default)]
    pub error: Option<serde_json::Value>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatRespMessage {
    #[serde(default)]
    pub id: String,
    #[serde(default)]
    pub author: ChatRespAuthor,
    #[serde(default)]
    pub content: ChatRespContent,
    #[serde(default)]
    pub status: String,
    #[serde(default)]
    pub metadata: ChatRespMetadata,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct ChatRespAuthor {
    #[serde(default)]
    pub role: String,
    #[serde(default)]
    pub name: Option<serde_json::Value>,
    #[serde(default)]
    pub metadata: serde_json::Value,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct ChatRespContent {
    #[serde(default)]
    pub content_type: String,
    #[serde(default)]
    pub parts: Vec<serde_json::Value>,
    #[serde(default)]
    pub language: String,
    #[serde(default)]
    pub text: String,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct ChatRespMetadata {
    #[serde(default)]
    pub message_type: String,
    #[serde(default)]
    pub finish_details: Option<FinishDetails>,
    #[serde(default)]
    pub model_slug: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FinishDetails {
    #[serde(rename = "types", default)]
    pub finish_type: String,
    #[serde(default)]
    pub stop: String,
}

// ===== Completions API Types =====

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ApiReq {
    #[serde(default)]
    pub messages: Vec<ApiMessage>,
    #[serde(default)]
    pub model: String,
    #[serde(default)]
    pub stream: bool,
    #[serde(default)]
    pub plugin_ids: Vec<String>,
    #[serde(default)]
    pub conversation_id: String,
    #[serde(default)]
    pub parent_message_id: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ApiMessage {
    #[serde(default)]
    pub role: String,
    #[serde(default)]
    pub content: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ApiRespJson {
    #[serde(skip_serializing_if = "String::is_empty")]
    pub id: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub object: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub created: Option<i64>,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub model: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub conversation_id: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub message_id: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub usage: Option<ApiRespJsonUsage>,
    #[serde(skip_serializing_if = "Vec::is_empty")]
    pub choices: Vec<ApiRespJsonChoice>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ApiRespJsonUsage {
    pub prompt_tokens: i32,
    pub completion_tokens: i32,
    pub total_tokens: i32,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ApiRespJsonChoice {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub delta: Option<ApiRespJsonChoiceDelta>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub message: Option<ApiRespJsonMessage>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub finish_reason: Option<String>,
    #[serde(default)]
    pub index: i32,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ApiRespJsonMessage {
    #[serde(skip_serializing_if = "String::is_empty")]
    pub role: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub content: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ApiRespJsonChoiceDelta {
    #[serde(skip_serializing_if = "String::is_empty")]
    pub content: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub role: String,
}

impl ApiRespJson {
    pub fn new(id: &str, model: &str, content: &str) -> Self {
        Self {
            id: id.to_string(),
            object: "chat.completion".to_string(),
            created: Some(chrono::Utc::now().timestamp()),
            model: model.to_string(),
            conversation_id: String::new(),
            message_id: String::new(),
            usage: Some(ApiRespJsonUsage {
                prompt_tokens: 0,
                completion_tokens: 0,
                total_tokens: 0,
            }),
            choices: vec![ApiRespJsonChoice {
                delta: None,
                message: Some(ApiRespJsonMessage {
                    role: "assistant".to_string(),
                    content: content.to_string(),
                }),
                finish_reason: Some("stop".to_string()),
                index: 0,
            }],
        }
    }
}

// ===== Stream Types =====

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ApiRespStream {
    #[serde(skip_serializing_if = "String::is_empty")]
    pub id: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub object: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub created: Option<i64>,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub model: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub conversation_id: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub message_id: String,
    #[serde(skip_serializing_if = "Vec::is_empty")]
    pub choices: Vec<ApiStreamChoice>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ApiStreamChoice {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub delta: Option<ApiStreamDelta>,
    #[serde(default)]
    pub index: i32,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub finish_reason: Option<serde_json::Value>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ApiStreamDelta {
    #[serde(skip_serializing_if = "String::is_empty")]
    pub content: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub role: String,
}

impl ApiRespStream {
    pub fn new(id: &str, model: &str, content: &str) -> Self {
        Self {
            id: id.to_string(),
            object: "chat.completion.chunk".to_string(),
            created: Some(chrono::Utc::now().timestamp()),
            model: model.to_string(),
            conversation_id: String::new(),
            message_id: String::new(),
            choices: vec![ApiStreamChoice {
                delta: Some(ApiStreamDelta {
                    content: content.to_string(),
                    role: String::new(),
                }),
                index: 0,
                finish_reason: None,
            }],
        }
    }

    pub fn stop_chunk(id: &str, model: &str, finish_reason: &str) -> Self {
        Self {
            id: id.to_string(),
            object: "chat.completion.chunk".to_string(),
            created: Some(chrono::Utc::now().timestamp()),
            model: model.to_string(),
            conversation_id: String::new(),
            message_id: String::new(),
            choices: vec![ApiStreamChoice {
                delta: None,
                index: 0,
                finish_reason: Some(serde_json::Value::String(finish_reason.to_string())),
            }],
        }
    }
}

// ===== Responses API Types =====

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponsesApiReq {
    #[serde(default)]
    pub model: String,
    #[serde(default)]
    pub input: Option<serde_json::Value>,
    #[serde(default)]
    pub instructions: String,
    #[serde(default)]
    pub stream: bool,
    #[serde(default)]
    pub tools: Vec<ResponsesTool>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponsesTool {
    #[serde(rename = "type", default)]
    pub tool_type: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponsesEvent {
    #[serde(rename = "type")]
    pub event_type: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub response: Option<ResponsesResponse>,
    #[serde(default)]
    pub output_index: i32,
    #[serde(default)]
    pub content_index: i32,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub item_id: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub item: Option<ResponsesOutputItem>,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub delta: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub text: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponsesResponse {
    pub id: String,
    pub object: String,
    pub created_at: i64,
    pub status: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error: Option<serde_json::Value>,
    #[serde(rename = "incomplete_details", skip_serializing_if = "Option::is_none")]
    pub incomplete_details: Option<serde_json::Value>,
    pub model: String,
    pub output: Vec<ResponsesOutputItem>,
    pub parallel_tool_calls: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponsesOutputItem {
    #[serde(skip_serializing_if = "String::is_empty")]
    pub id: String,
    #[serde(rename = "type")]
    pub item_type: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub status: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub role: String,
    #[serde(skip_serializing_if = "Vec::is_empty")]
    pub content: Vec<ResponsesContentPart>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponsesContentPart {
    #[serde(rename = "type")]
    pub part_type: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub text: String,
    #[serde(default)]
    pub annotations: Vec<serde_json::Value>,
}

impl ResponsesOutputItem {
    pub fn text_output_item(id: &str, text: &str, status: &str) -> Self {
        Self {
            id: id.to_string(),
            item_type: "message".to_string(),
            status: status.to_string(),
            role: "assistant".to_string(),
            content: vec![ResponsesContentPart {
                part_type: "output_text".to_string(),
                text: text.to_string(),
                annotations: vec![],
            }],
        }
    }
}

impl ResponsesEvent {
    pub fn created_event(response_id: &str, model: &str, created: i64) -> Self {
        Self {
            event_type: "response.created".to_string(),
            response: Some(ResponsesResponse {
                id: response_id.to_string(),
                object: "response".to_string(),
                created_at: created,
                status: "in_progress".to_string(),
                error: None,
                incomplete_details: None,
                model: model.to_string(),
                output: vec![],
                parallel_tool_calls: false,
            }),
            output_index: 0,
            content_index: 0,
            item_id: String::new(),
            item: None,
            delta: String::new(),
            text: String::new(),
        }
    }

    pub fn completed_event(
        response_id: &str,
        model: &str,
        created: i64,
        output: Vec<ResponsesOutputItem>,
    ) -> Self {
        Self {
            event_type: "response.completed".to_string(),
            response: Some(ResponsesResponse {
                id: response_id.to_string(),
                object: "response".to_string(),
                created_at: created,
                status: "completed".to_string(),
                error: None,
                incomplete_details: None,
                model: model.to_string(),
                output,
                parallel_tool_calls: false,
            }),
            output_index: 0,
            content_index: 0,
            item_id: String::new(),
            item: None,
            delta: String::new(),
            text: String::new(),
        }
    }

    pub fn to_sse(&self) -> String {
        let data = serde_json::to_string(self).unwrap_or_default();
        format!("data: {}\n\n", data)
    }
}

pub fn response_id() -> String {
    format!("resp_{}", chrono::Utc::now().timestamp_nanos_opt().unwrap_or(0))
}

pub fn message_id() -> String {
    format!("msg_{}", chrono::Utc::now().timestamp_nanos_opt().unwrap_or(0))
}

pub fn normalize_model(model: &str) -> String {
    let m = model.trim();
    if m.is_empty() {
        "auto".to_string()
    } else {
        m.to_string()
    }
}
