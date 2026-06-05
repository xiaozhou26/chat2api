use axum::{
    extract::{Request, State},
    http::HeaderValue,
    middleware::Next,
    response::Response,
};
use std::sync::{Arc, RwLock};

use crate::conf::AppConfig;

/// V1 Auth 中间件
pub async fn v1_auth(
    State(config): State<Arc<RwLock<AppConfig>>>,
    req: Request,
    next: Next,
) -> Response {
    let auth_header = req
        .headers()
        .get("authorization")
        .and_then(|v| v.to_str().ok())
        .unwrap_or("");

    let local_token = auth_header
        .trim()
        .trim_start_matches("Bearer ")
        .trim();

    // 直传 at- token，跳过认证
    if local_token.starts_with("at-") {
        return next.run(req).await;
    }

    // 直传 JWT token
    if auth_header.starts_with("Bearer eyJhbGciOiJSUzI1NiI") {
        return next.run(req).await;
    }

    let access_tokens = {
        match config.read() {
            Ok(guard) => guard.auth.access_tokens.clone(),
            Err(_) => Vec::new(),
        }
    };

    // 无 Authorization 且有配置的 token
    if auth_header.is_empty() && !access_tokens.is_empty() {
        return axum::response::IntoResponse::into_response(
            (axum::http::StatusCode::UNAUTHORIZED,
             axum::Json(serde_json::json!({
                 "detail": {
                     "code": 401,
                     "msg": "You didn't provide an API key. You need to provide your API key in an Authorization header using Bearer auth (i.e. Authorization: Bearer YOUR_KEY)",
                     "error": null
                 }
             })))
        );
    }

    // 校验 token
    if !access_tokens.is_empty() && !access_tokens.iter().any(|t| t == local_token) {
        return axum::response::IntoResponse::into_response(
            (axum::http::StatusCode::UNAUTHORIZED,
             axum::Json(serde_json::json!({
                 "detail": {
                     "code": 401,
                     "msg": "Incorrect API key provided: sk-4yNZz***************************************6mjw.",
                     "error": null
                 }
             })))
        );
    }

    next.run(req).await
}

/// CORS 中间件
pub async fn v1_cors(req: Request, next: Next) -> Response {
    let mut response = next.run(req).await;
    let headers = response.headers_mut();
    headers.insert("Access-Control-Allow-Origin", HeaderValue::from_static("*"));
    headers.insert("Access-Control-Allow-Credentials", HeaderValue::from_static("true"));
    headers.insert(
        "Access-Control-Allow-Headers",
        HeaderValue::from_static("Authorization, Token, Content-Type, Accept"),
    );
    headers.insert(
        "Access-Control-Allow-Methods",
        HeaderValue::from_static("GET, POST, PUT, DELETE, OPTIONS"),
    );
    response
}
