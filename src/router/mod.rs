use crate::conf::AppConfig;
use crate::middleware;
use crate::service;
use axum::{
    Router,
    routing::{get, post},
    Json,
};
use std::sync::{Arc, RwLock};
use tokio::sync::broadcast;
use tower_http::cors::{CorsLayer, Any};

async fn ping() -> Json<serde_json::Value> {
    Json(serde_json::json!({"message": "pong"}))
}

async fn index() -> &'static str {
    "hello, this is chat2api."
}

pub fn create_app(config: Arc<RwLock<AppConfig>>) -> Router {
    let v1 = Router::new()
        .route("/accTokens", get(service::acc_tokens))
        .route("/chat/completions", post(service::completions))
        .route("/responses", post(service::responses))
        .layer(axum::middleware::from_fn_with_state(
            config.clone(),
            middleware::v1_auth,
        ))
        .layer(axum::middleware::from_fn(middleware::v1_cors))
        .with_state(config);

    Router::new()
        .route("/", get(index))
        .route("/ping", get(ping))
        .nest("/v1", v1)
        .layer(
            CorsLayer::new()
                .allow_origin(Any)
                .allow_methods(Any)
                .allow_headers(Any),
        )
}

pub fn start(config: Arc<RwLock<AppConfig>>, mut shutdown: broadcast::Receiver<()>) -> tokio::task::JoinHandle<()> {
    let addr = {
        match config.read() {
            Ok(guard) => format!("{}:{}", guard.bind, guard.port),
            Err(_) => "0.0.0.0:3040".to_string(),
        }
    };

    let app = create_app(config);

    tokio::spawn(async move {
        let listener = match tokio::net::TcpListener::bind(&addr).await {
            Ok(l) => l,
            Err(e) => {
                tracing::error!("failed to bind {}: {}", addr, e);
                return;
            }
        };
        tracing::info!("httpServer started on http://{}", addr);

        axum::serve(listener, app)
            .with_graceful_shutdown(async move {
                let _ = shutdown.recv().await;
            })
            .await
            .unwrap_or_else(|e| {
                tracing::error!("server error: {}", e);
            });

        tracing::info!("http server is shutting down complete");
    })
}
