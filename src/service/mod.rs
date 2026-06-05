use crate::acc_token_pool;
use crate::chat_backend;
use crate::chat_backend::build_chat_request;
use crate::types::*;
use axum::{
    Json,
    http::StatusCode,
    response::IntoResponse,
};
use std::sync::{Arc, RwLock};
use crate::conf::AppConfig;

/// 查看账号池
pub async fn acc_tokens(
    axum::extract::State(_config): axum::extract::State<Arc<RwLock<AppConfig>>>,
) -> impl IntoResponse {
    let pool = acc_token_pool::pool();
    let resp = serde_json::json!({
        "count": pool.size(),
        "can_use_count": pool.can_use_size(),
    });
    tracing::info!("AccessTokenPool Tokens: {}", pool.size());
    (StatusCode::OK, Json(resp))
}

/// Chat Completions
pub async fn completions(
    axum::extract::State(_config): axum::extract::State<Arc<RwLock<AppConfig>>>,
    headers: axum::http::HeaderMap,
    Json(api_req): Json<ApiReq>,
) -> impl IntoResponse {
    if api_req.model.trim().is_empty() && api_req.messages.is_empty() {
        return error_response(StatusCode::BAD_REQUEST, "Invalid parameter");
    }

    let chat_req = build_chat_request(&api_req);
    if chat_req.model.is_empty() {
        return error_response(StatusCode::BAD_REQUEST, "Model is unsupported");
    }

    let auth_token = headers
        .get("authorization")
        .and_then(|v| v.to_str().ok())
        .unwrap_or("");

    let backend = match chat_backend::Backend::new(auth_token, chat_backend::retry()).await {
        Ok(b) => b,
        Err(e) => {
            tracing::error!("backend init failed: {}", e);
            return error_response(StatusCode::BAD_GATEWAY, &e);
        }
    };

    let resp = match backend.send_chat_request(&chat_req).await {
        Ok(r) => r,
        Err(e) => {
            tracing::error!("upstream request failed: {}", e);
            return error_response(StatusCode::BAD_GATEWAY, &e);
        }
    };

    let status = resp.status();
    if status.as_u16() != 200 {
        let status_code = StatusCode::from_u16(status.as_u16()).unwrap_or(StatusCode::BAD_GATEWAY);
        let body = resp.text().await.unwrap_or_default();

        // 处理 429 限速
        if status.as_u16() == 429 {
            let can_use_at = rate_limit_can_use_at(&body);
            acc_token_pool::pool().set_can_use_at(backend.acc_auth(), can_use_at);
        }

        let detail = parse_error_detail(&body);
        return error_response(status_code, &detail);
    }

    // 处理 SSE 流式响应
    let is_stream = api_req.stream;
    let model = api_req.model.clone();
    let id = chat_backend::generate_completion_id(29);

    match handle_sse_response(resp, &id, &model, is_stream).await {
        Ok(result) => {
            if is_stream {
                // 流式响应已在 handle_sse_response 中处理
                (
                    StatusCode::OK,
                    [(
                        "content-type".to_string(),
                        "text/event-stream".to_string(),
                    )],
                    result.content,
                )
                    .into_response()
            } else {
                let mut resp_json = ApiRespJson::new(&id, &model, &result.content);
                resp_json.conversation_id = result.conversation_id;
                resp_json.message_id = result.message_id;
                (StatusCode::OK, Json(resp_json)).into_response()
            }
        }
        Err(e) => {
            tracing::error!("handle response failed: {}", e);
            error_response(StatusCode::BAD_GATEWAY, &e)
        }
    }
}

/// Responses API
pub async fn responses(
    axum::extract::State(_config): axum::extract::State<Arc<RwLock<AppConfig>>>,
    headers: axum::http::HeaderMap,
    Json(api_req): Json<ResponsesApiReq>,
) -> impl IntoResponse {
    // 检查 image_generation 工具
    if has_image_generation_tool(&api_req.tools) {
        return error_response(
            StatusCode::NOT_IMPLEMENTED,
            "responses image_generation tool is not implemented",
        );
    }

    let comp_messages = response_messages(&api_req);
    if comp_messages.is_empty() {
        return error_response(StatusCode::BAD_REQUEST, "input text is required");
    }

    let comp_req = ApiReq {
        model: normalize_model(&api_req.model),
        stream: false,
        messages: comp_messages,
        plugin_ids: vec![],
        conversation_id: String::new(),
        parent_message_id: String::new(),
    };

    let auth_token = headers
        .get("authorization")
        .and_then(|v| v.to_str().ok())
        .unwrap_or("");

    let backend = match chat_backend::Backend::new(auth_token, chat_backend::retry()).await {
        Ok(b) => b,
        Err(e) => {
            tracing::error!("backend init failed: {}", e);
            return error_response(StatusCode::BAD_GATEWAY, &e);
        }
    };

    let chat_req = build_chat_request(&comp_req);
    let resp = match backend.send_chat_request(&chat_req).await {
        Ok(r) => r,
        Err(e) => {
            tracing::error!("upstream request failed: {}", e);
            return error_response(StatusCode::BAD_GATEWAY, &e);
        }
    };

    let status = resp.status();
    if status.as_u16() != 200 {
        let status_code = StatusCode::from_u16(status.as_u16()).unwrap_or(StatusCode::BAD_GATEWAY);
        let body = resp.text().await.unwrap_or_default();
        if status.as_u16() == 429 {
            let can_use_at = rate_limit_can_use_at(&body);
            acc_token_pool::pool().set_can_use_at(backend.acc_auth(), can_use_at);
        }
        let detail = parse_error_detail(&body);
        return error_response(status_code, &detail);
    }

    let id = chat_backend::generate_completion_id(29);
    let model = comp_req.model.clone();

    match handle_sse_response(resp, &id, &model, false).await {
        Ok(result) => {
            if api_req.stream {
                // 流式返回 Responses 事件
                let sse_body = build_responses_sse(&model, &result.content);
                (
                    StatusCode::OK,
                    [("content-type".to_string(), "text/event-stream".to_string())],
                    sse_body,
                )
                    .into_response()
            } else {
                let item = ResponsesOutputItem::text_output_item(
                    &message_id(),
                    &result.content,
                    "completed",
                );
                let event = ResponsesEvent::completed_event(
                    &response_id(),
                    &model,
                    chrono::Utc::now().timestamp(),
                    vec![item],
                );
                (StatusCode::OK, Json(event.response.unwrap())).into_response()
            }
        }
        Err(e) => {
            tracing::error!("handle response failed: {}", e);
            error_response(StatusCode::BAD_GATEWAY, &e)
        }
    }
}

// ===== 辅助函数 =====

struct ChatResult {
    content: String,
    conversation_id: String,
    message_id: String,
}

async fn handle_sse_response(
    resp: wreq::Response,
    id: &str,
    model: &str,
    is_stream: bool,
) -> Result<ChatResult, String> {
    let body = resp
        .text()
        .await
        .map_err(|e| format!("read response body failed: {}", e))?;

    let mut previous_text = String::new();
    let mut finish_reason = String::new();
    let mut is_role = true;
    let mut conversation_id = String::new();
    let mut message_id = String::new();
    let mut stream_output = String::new();

    for line in body.lines() {
        let line = line.trim();
        if !line.starts_with("data: ") {
            continue;
        }
        let payload = line.trim_start_matches("data: ").trim();
        if payload.is_empty() {
            continue;
        }
        if payload == "[DONE]" {
            break;
        }

        let chat_resp: ChatResp = match serde_json::from_str(payload) {
            Ok(r) => r,
            Err(_) => continue,
        };

        if chat_resp.error.is_some() {
            return Err(format!("chatgpt error: {:?}", chat_resp.error));
        }

        if chat_resp.message.author.role != "assistant" || chat_resp.message.content.parts.is_empty() {
            continue;
        }

        if !chat_resp.conversation_id.is_empty() {
            conversation_id = chat_resp.conversation_id.clone();
        }
        if !chat_resp.message.id.is_empty() {
            message_id = chat_resp.message.id.clone();
        }

        let msg_type = &chat_resp.message.metadata.message_type;
        if !msg_type.is_empty() && msg_type != "next" && msg_type != "continue" {
            continue;
        }

        let content_type = &chat_resp.message.content.content_type;
        if !content_type.is_empty() && !content_type.ends_with("text") {
            continue;
        }

        // 获取文本内容
        let text = match chat_resp.message.content.parts.first() {
            Some(serde_json::Value::String(s)) => s.clone(),
            _ => continue,
        };

        let delta = text.trim_start_matches(&previous_text).to_string();
        previous_text = text;

        if is_stream {
            let mut stream_resp = ApiRespStream::new(id, model, &delta);
            stream_resp.conversation_id = conversation_id.clone();
            stream_resp.message_id = message_id.clone();
            if is_role {
                if let Some(ref mut delta) = stream_resp.choices[0].delta {
                    delta.role = chat_resp.message.author.role.clone();
                }
                is_role = false;
            }
            let data = serde_json::to_string(&stream_resp).unwrap_or_default();
            stream_output.push_str(&format!("data: {}\n\n", data));
        }

        if let Some(ref details) = chat_resp.message.metadata.finish_details {
            finish_reason = details.finish_type.clone();
        }
    }

    if is_stream {
        let stop = ApiRespStream::stop_chunk(id, model, &finish_reason);
        let stop_data = serde_json::to_string(&stop).unwrap_or_default();
        stream_output.push_str(&format!("data: {}\n\n", stop_data));
        stream_output.push_str("data: [DONE]\n\n");

        Ok(ChatResult {
            content: stream_output,
            conversation_id,
            message_id,
        })
    } else {
        Ok(ChatResult {
            content: previous_text,
            conversation_id,
            message_id,
        })
    }
}

fn error_response(status: StatusCode, msg: &str) -> axum::response::Response {
    (
        status,
        Json(serde_json::json!({
            "detail": {
                "code": status.as_u16() as i32,
                "msg": msg,
                "error": null
            }
        })),
    )
        .into_response()
}

fn parse_error_detail(body: &str) -> String {
    if let Ok(v) = serde_json::from_str::<serde_json::Value>(body) {
        if let Some(detail) = v.get("detail") {
            if detail.is_string() {
                return detail.as_str().unwrap_or(body).to_string();
            }
            return detail.to_string();
        }
    }
    body.chars().take(4096).collect()
}

fn rate_limit_can_use_at(body: &str) -> i64 {
    let now = chrono::Utc::now().timestamp();
    if let Ok(v) = serde_json::from_str::<serde_json::Value>(body) {
        if let Some(t) = find_rate_limit_time(&v, now) {
            return t;
        }
    }
    now + 3600 // 默认 1 小时
}

fn find_rate_limit_time(value: &serde_json::Value, now: i64) -> Option<i64> {
    match value {
        serde_json::Value::Object(map) => {
            for key in &["retry_after", "reset_after", "resets_after", "restore_at", "reset_at"] {
                if let Some(candidate) = map.get(*key) {
                    if let Some(parsed) = parse_rate_limit_value(candidate, now) {
                        return Some(parsed);
                    }
                }
            }
            for (_, child) in map {
                if let Some(parsed) = find_rate_limit_time(child, now) {
                    return Some(parsed);
                }
            }
            None
        }
        serde_json::Value::Array(arr) => {
            for child in arr {
                if let Some(parsed) = find_rate_limit_time(child, now) {
                    return Some(parsed);
                }
            }
            None
        }
        _ => None,
    }
}

fn parse_rate_limit_value(value: &serde_json::Value, now: i64) -> Option<i64> {
    match value {
        serde_json::Value::Number(n) => {
            let v = n.as_f64()? as i64;
            Some(normalize_rate_limit_unix(v, now))
        }
        serde_json::Value::String(s) => {
            if let Ok(seconds) = s.parse::<i64>() {
                Some(normalize_rate_limit_unix(seconds, now))
            } else {
                None
            }
        }
        _ => None,
    }
}

fn normalize_rate_limit_unix(value: i64, now: i64) -> i64 {
    if value <= 0 {
        return 0;
    }
    if value < 30 * 24 * 3600 {
        now + value
    } else {
        value
    }
}

fn response_messages(req: &ResponsesApiReq) -> Vec<ApiMessage> {
    let mut messages = Vec::new();

    let instructions = req.instructions.trim();
    if !instructions.is_empty() {
        messages.push(ApiMessage {
            role: "system".to_string(),
            content: instructions.to_string(),
        });
    }

    if has_non_image_tools(&req.tools) {
        messages.push(ApiMessage {
            role: "system".to_string(),
            content: "This compatibility backend cannot execute local tools, shell commands, web searches, or file operations. Do not claim to have run tools or inspected external resources. If a user asks you to use a tool, say that tool execution is unavailable through this backend.".to_string(),
        });
    }

    if let Some(ref input) = req.input {
        messages.extend(input_messages(input));
    }

    messages
}

fn input_messages(input: &serde_json::Value) -> Vec<ApiMessage> {
    match input {
        serde_json::Value::String(s) => {
            if s.trim().is_empty() {
                vec![]
            } else {
                vec![ApiMessage {
                    role: "user".to_string(),
                    content: s.trim().to_string(),
                }]
            }
        }
        serde_json::Value::Object(map) => {
            let role = map
                .get("role")
                .and_then(|v| v.as_str())
                .unwrap_or("user")
                .to_string();
            let content = message_content_text(map);
            vec![ApiMessage { role, content }]
        }
        serde_json::Value::Array(arr) => {
            let mut messages = Vec::new();
            for item in arr {
                if let Some(map) = item.as_object() {
                    let text = message_content_text(map);
                    if !text.trim().is_empty() {
                        let role = map
                            .get("role")
                            .and_then(|v| v.as_str())
                            .unwrap_or("user")
                            .to_string();
                        messages.push(ApiMessage { role, content: text });
                    }
                }
            }
            messages
        }
        _ => vec![],
    }
}

fn message_content_text(map: &serde_json::Map<String, serde_json::Value>) -> String {
    if let Some(s) = map.get("text").and_then(|v| v.as_str()) {
        if !s.is_empty() {
            return s.to_string();
        }
    }
    if let Some(s) = map.get("content").and_then(|v| v.as_str()) {
        if !s.is_empty() {
            return s.to_string();
        }
    }
    if let Some(arr) = map.get("content").and_then(|v| v.as_array()) {
        let parts: Vec<String> = arr
            .iter()
            .filter_map(|raw| {
                raw.as_object()
                    .and_then(|p| p.get("text").and_then(|t| t.as_str()).map(|s| s.to_string()))
            })
            .collect();
        if !parts.is_empty() {
            return parts.join("");
        }
    }
    if is_content_part(map) {
        return map
            .get("text")
            .and_then(|v| v.as_str())
            .unwrap_or("")
            .to_string();
    }
    serde_json::to_string(map).unwrap_or_default()
}

fn is_content_part(map: &serde_json::Map<String, serde_json::Value>) -> bool {
    matches!(
        map.get("type").and_then(|v| v.as_str()).unwrap_or(""),
        "text" | "input_text" | "output_text"
    )
}

fn has_image_generation_tool(tools: &[ResponsesTool]) -> bool {
    tools
        .iter()
        .any(|t| t.tool_type.trim() == "image_generation")
}

fn has_non_image_tools(tools: &[ResponsesTool]) -> bool {
    tools
        .iter()
        .any(|t| !t.tool_type.trim().is_empty() && t.tool_type.trim() != "image_generation")
}

fn build_responses_sse(model: &str, text: &str) -> String {
    let response_id = response_id();
    let item_id = message_id();
    let created = chrono::Utc::now().timestamp();

    let mut output = String::new();

    output.push_str(&ResponsesEvent::created_event(&response_id, model, created).to_sse());

    let mut item = ResponsesOutputItem::text_output_item("", "", "in_progress");
    item.id = item_id.clone();
    output.push_str(
        &ResponsesEvent {
            event_type: "response.output_item.added".to_string(),
            response: None,
            output_index: 0,
            content_index: 0,
            item_id: String::new(),
            item: Some(item),
            delta: String::new(),
            text: String::new(),
        }
        .to_sse(),
    );

    output.push_str(
        &ResponsesEvent {
            event_type: "response.output_text.delta".to_string(),
            response: None,
            output_index: 0,
            content_index: 0,
            item_id: item_id.clone(),
            item: None,
            delta: text.to_string(),
            text: String::new(),
        }
        .to_sse(),
    );

    output.push_str(
        &ResponsesEvent {
            event_type: "response.output_text.done".to_string(),
            response: None,
            output_index: 0,
            content_index: 0,
            item_id: item_id.clone(),
            item: None,
            delta: String::new(),
            text: text.to_string(),
        }
        .to_sse(),
    );

    let completed_item = ResponsesOutputItem::text_output_item(&item_id, text, "completed");
    output.push_str(
        &ResponsesEvent {
            event_type: "response.output_item.done".to_string(),
            response: None,
            output_index: 0,
            content_index: 0,
            item_id: String::new(),
            item: Some(completed_item.clone()),
            delta: String::new(),
            text: String::new(),
        }
        .to_sse(),
    );

    output.push_str(
        &ResponsesEvent::completed_event(
            &response_id,
            model,
            created,
            vec![completed_item],
        )
        .to_sse(),
    );

    output.push_str("data: [DONE]\n\n");
    output
}
