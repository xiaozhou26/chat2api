use crate::acc_token_pool;
use crate::conf;
use crate::proof_work;
use crate::turnstile;
use crate::types;
use crate::types::ApiReq;
use rand::Rng;
use serde::Deserialize;
use std::collections::HashMap;
use uuid::Uuid;
use wreq::Client;

const RETRY: i32 = 3;

#[derive(Debug, Clone, Deserialize)]
struct ChatRequirements {
    #[serde(default)]
    arkose: Challenge,
    #[serde(default)]
    turnstile: Challenge,
    #[serde(rename = "proofofwork")]
    proof_work: proof_work::ProofWork,
    #[serde(default)]
    token: String,
    #[serde(rename = "so_token", default)]
    so_token: String,
    #[serde(rename = "force_login", default)]
    force_login: bool,
}

#[derive(Debug, Clone, Default, Deserialize)]
struct Challenge {
    #[serde(default)]
    required: bool,
    #[serde(default)]
    dx: String,
}

pub struct Backend {
    client: Client,
    auth: ChatRequirements,
    acc_auth: String,
    base_url: String,
    chat_url: String,
    user_agent: String,
    session_id: String,
    oai_device_id: String,
    pow: proof_work::Resources,
}

impl Backend {
    pub fn new(token: &str, retry: i32) -> std::pin::Pin<Box<dyn std::future::Future<Output = Result<Self, String>> + Send + '_>> {
        Box::pin(async move {
            let token = token.trim();
            let local_token = token.trim_start_matches("Bearer ").trim();

            if local_token.starts_with("at-") {
                return Self::new_backend(
                    &format!("Bearer {}", &local_token[3..]),
                    "",
                )
                .await;
            }

            if token.starts_with("Bearer eyJhbGciOiJSUzI1NiI") {
                return Self::new_backend(token, "").await;
            }

            if !acc_token_pool::pool().is_empty() {
                if let Some(access_token) = acc_token_pool::pool().get_access_token() {
                    match Self::new_backend(&access_token.token, &access_token.proxy).await {
                        Ok(backend) => return Ok(backend),
                        Err(_) if retry > 0 => {
                            return Self::new(token, retry - 1).await;
                        }
                        Err(e) => return Err(e),
                    }
                }
                return Err("access token pool is empty".to_string());
            }

            if local_token.starts_with("sk-") {
                return Err("access token pool is empty".to_string());
            }

            match Self::new_backend(token, "").await {
                Ok(backend) => Ok(backend),
                Err(e) if retry > 0 => Self::new(token, retry - 1).await,
                Err(e) => Err(e),
            }
        })
    }

    async fn new_backend(token: &str, account_proxy: &str) -> Result<Self, String> {
        let base_url = conf::load_config()
            .chatgpt_base_url
            .trim_end_matches('/')
            .to_string();
        let base_url = if base_url.is_empty() {
            "https://chatgpt.com".to_string()
        } else {
            base_url
        };

        let user_agent = get_ua();
        let session_id = uuid::Uuid::new_v4().to_string();

        // 构建 wreq 客户端，使用浏览器指纹模拟
        let mut client_builder = Client::builder()
            .timeout(std::time::Duration::from_secs(300))
            .emulation(wreq_util::Emulation::Chrome136);

        // 设置代理
        let proxy = {
            let proxy = account_proxy.trim().to_string();
            if proxy.is_empty() {
                conf::load_config().proxy.trim().to_string()
            } else {
                proxy
            }
        };

        if !proxy.is_empty() {
            let wreq_proxy = wreq::Proxy::all(&proxy)
                .map_err(|e| format!("parse proxy failed: {}", e))?;
            client_builder = client_builder.proxy(wreq_proxy);
        }

        let client = client_builder
            .build()
            .map_err(|e| format!("build client failed: {}", e))?;

        let mut chat_url = format!("{}/backend-anon/conversation", base_url);
        let mut acc_auth = String::new();

        if token.starts_with("Bearer ") {
            acc_auth = token.to_string();
            chat_url = format!("{}/backend-api/conversation", base_url);
        }

        let oai_device_id = uuid::Uuid::new_v4().to_string();

        let mut backend = Self {
            client,
            auth: ChatRequirements {
                arkose: Challenge::default(),
                turnstile: Challenge::default(),
                proof_work: proof_work::ProofWork {
                    difficulty: String::new(),
                    required: false,
                    seed: String::new(),
                    ospt: String::new(),
                },
                token: String::new(),
                so_token: String::new(),
                force_login: false,
            },
            acc_auth,
            base_url,
            chat_url,
            user_agent,
            session_id,
            oai_device_id: oai_device_id.clone(),
            pow: proof_work::Resources::default(),
        };

        // 加载 PoW 资源
        backend.load_pow_resources(&oai_device_id).await;

        // 加载 chat requirements
        backend
            .load_requirements(&oai_device_id)
            .await?;

        Ok(backend)
    }

    pub fn build_headers(&self, url: &str) -> HashMap<String, String> {
        let mut headers = HashMap::new();
        let path = url.trim_start_matches(&self.base_url);

        headers.insert("accept".into(), "*/*".into());
        headers.insert("accept-language".into(), "zh-CN,zh;q=0.9,en;q=0.8,en-US;q=0.7".into());
        headers.insert("origin".into(), self.base_url.clone());
        headers.insert("referer".into(), format!("{}/", self.base_url));
        headers.insert("cache-control".into(), "no-cache".into());
        headers.insert("pragma".into(), "no-cache".into());
        headers.insert("priority".into(), "u=1, i".into());
        headers.insert(
            "sec-ch-ua".into(),
            r#""Microsoft Edge";v="143", "Chromium";v="143", "Not A(Brand";v="24""#.into(),
        );
        headers.insert("sec-ch-ua-arch".into(), r#""x86""#.into());
        headers.insert("sec-ch-ua-bitness".into(), r#""64""#.into());
        headers.insert(
            "sec-ch-ua-full-version".into(),
            r#""143.0.3650.96""#.into(),
        );
        headers.insert(
            "sec-ch-ua-full-version-list".into(),
            r#""Microsoft Edge";v="143.0.3650.96", "Chromium";v="143.0.7499.147", "Not A(Brand";v="24.0.0.0""#
                .into(),
        );
        headers.insert("sec-ch-ua-mobile".into(), "?0".into());
        headers.insert("sec-ch-ua-model".into(), r#""""#.into());
        headers.insert("sec-ch-ua-platform".into(), r#""Windows""#.into());
        headers.insert("sec-ch-ua-platform-version".into(), r#""19.0.0""#.into());
        headers.insert("sec-fetch-dest".into(), "empty".into());
        headers.insert("sec-fetch-mode".into(), "cors".into());
        headers.insert("sec-fetch-site".into(), "same-origin".into());
        headers.insert("user-agent".into(), self.user_agent.clone());
        headers.insert("oai-device-id".into(), self.oai_device_id.clone());
        headers.insert("oai-session-id".into(), self.session_id.clone());
        headers.insert("oai-language".into(), "zh-CN".into());
        headers.insert(
            "oai-client-version".into(),
            "prod-3b8f2c1740596d77c64c1d3d50205828839b2730".into(),
        );
        headers.insert("oai-client-build-number".into(), "3310101057".into());
        headers.insert("x-openai-target-path".into(), path.into());
        headers.insert("x-openai-target-route".into(), path.into());

        if !self.acc_auth.is_empty() {
            headers.insert("authorization".into(), self.acc_auth.clone());
        }

        headers
    }

    async fn load_pow_resources(&mut self, oai_device_id: &str) {
        let mut headers = self.build_headers(&format!("{}/", self.base_url));
        headers.insert("accept".into(), "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8".into());
        headers.insert("oai-device-id".into(), oai_device_id.to_string());

        let mut req = self.client.get(&format!("{}/", self.base_url));
        for (k, v) in &headers {
            req = req.header(k.as_str(), v.as_str());
        }

        match req.send().await {
            Ok(resp) => {
                if let Ok(body) = resp.text().await {
                    self.pow = proof_work::parse_resources(&body);
                }
            }
            Err(e) => {
                tracing::warn!("load pow resources failed: {}", e);
            }
        }
    }

    async fn load_requirements(&mut self, oai_device_id: &str) -> Result<(), String> {
        let auth_url = if self.acc_auth.is_empty() {
            format!("{}/backend-anon/sentinel/chat-requirements", self.base_url)
        } else {
            format!("{}/backend-api/sentinel/chat-requirements", self.base_url)
        };

        let requirements_token =
            proof_work::legacy_requirements_token(&self.user_agent, Some(&self.pow.clone()));
        let body = format!(r#"{{"p":"{}"}}"#, requirements_token);

        let mut headers = self.build_headers(&auth_url);
        headers.insert("content-type".into(), "application/json".into());
        headers.insert("oai-device-id".into(), oai_device_id.to_string());

        let mut req = self.client.post(&auth_url);
        for (k, v) in &headers {
            req = req.header(k.as_str(), v.as_str());
        }
        req = req.body(body);

        let resp = req
            .send()
            .await
            .map_err(|e| format!("chat requirements request failed: {}", e))?;

        let status = resp.status().as_u16();
        if status != 200 {
            let detail = resp.text().await.unwrap_or_default();
            return Err(format!(
                "chat requirements failed: status={} body={}",
                status,
                &detail[..detail.len().min(4096)]
            ));
        }

        let body = resp
            .text()
            .await
            .map_err(|e| format!("read requirements body failed: {}", e))?;

        let mut auth: ChatRequirements =
            serde_json::from_str(&body).map_err(|e| format!("parse requirements failed: {}", e))?;

        if auth.force_login {
            return Err("force login required".to_string());
        }

        if auth.arkose.required {
            return Err("arkose token is required".to_string());
        }

        if auth.turnstile.required && !auth.turnstile.dx.is_empty() {
            let source_p = if self.acc_auth.is_empty() {
                requirements_token.clone()
            } else {
                String::new()
            };
            auth.turnstile.dx = turnstile::solve(&auth.turnstile.dx, &source_p);
            if auth.turnstile.dx.is_empty() {
                let fallback_p = if source_p == requirements_token {
                    String::new()
                } else {
                    requirements_token
                };
                auth.turnstile.dx = turnstile::solve(&auth.turnstile.dx, &fallback_p);
            }
        }

        if auth.proof_work.required {
            auth.proof_work.ospt = proof_work::calc_proof_token(
                &auth.proof_work.seed,
                &auth.proof_work.difficulty,
                &self.user_agent,
                Some(&self.pow),
            );
            if auth.proof_work.ospt.is_empty() {
                return Err("proof token failed".to_string());
            }
        }

        if auth.token.is_empty() {
            return Err("missing chat requirements token".to_string());
        }

        self.auth = auth;
        Ok(())
    }

    /// 发送聊天请求，返回原始响应
    pub async fn send_chat_request(
        &self,
        chat_req: &types::ChatReq,
    ) -> Result<wreq::Response, String> {
        let body = serde_json::to_string(chat_req)
            .map_err(|e| format!("serialize chat request failed: {}", e))?;

        let mut headers = self.build_headers(&self.chat_url);
        headers.insert("accept".into(), "text/event-stream".into());
        headers.insert("content-type".into(), "application/json".into());
        headers.insert(
            "openai-sentinel-chat-requirements-token".into(),
            self.auth.token.clone(),
        );
        if !self.auth.proof_work.ospt.is_empty() {
            headers.insert(
                "openai-sentinel-proof-token".into(),
                self.auth.proof_work.ospt.clone(),
            );
        }
        if !self.auth.turnstile.dx.is_empty() && self.auth.turnstile.required {
            headers.insert(
                "openai-sentinel-turnstile-token".into(),
                self.auth.turnstile.dx.clone(),
            );
        }
        if !self.auth.so_token.is_empty() {
            headers.insert(
                "openai-sentinel-so-token".into(),
                self.auth.so_token.clone(),
            );
        }

        let mut req = self.client.post(&self.chat_url);
        for (k, v) in &headers {
            req = req.header(k.as_str(), v.as_str());
        }
        req = req.body(body);

        req.send()
            .await
            .map_err(|e| format!("upstream request failed: {}", e))
    }

    pub fn acc_auth(&self) -> &str {
        &self.acc_auth
    }

    pub fn auth_token(&self) -> &str {
        &self.auth.token
    }

    pub fn proof_token(&self) -> &str {
        &self.auth.proof_work.ospt
    }

    pub fn turnstile_token(&self) -> &str {
        &self.auth.turnstile.dx
    }

    pub fn so_token(&self) -> &str {
        &self.auth.so_token
    }
}

pub fn retry() -> i32 {
    RETRY
}

fn get_ua() -> String {
    let user_agents = [
        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.6935.0 Safari/537.36 Edg/136.0.0.0",
        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.6935.0 Safari/537.36",
        "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.6935.0 Safari/537.36",
        "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.6935.0 Safari/537.36",
    ];
    let mut rng = rand::thread_rng();
    user_agents[rng.gen_range(0..user_agents.len())].to_string()
}

/// 生成 chatcmpl- 风格的 ID
pub fn generate_completion_id(length: usize) -> String {
    const CHARSET: &[u8] = b"abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789";
    let mut rng = rand::thread_rng();
    let id: String = (0..length)
        .map(|_| CHARSET[rng.gen_range(0..CHARSET.len())] as char)
        .collect();
    format!("chatcmpl-{}", id)
}

/// 将 API 请求转换为 ChatGPT 后端请求格式
pub fn build_chat_request(api_req: &ApiReq) -> types::ChatReq {
    let messages: Vec<types::ChatMessages> = api_req
        .messages
        .iter()
        .map(|api_message| types::ChatMessages {
            id: Uuid::new_v4().to_string(),
            author: types::ChatAuthor {
                role: api_message.role.clone(),
            },
            content: types::ChatContent {
                content_type: "text".to_string(),
                parts: vec![api_message.content.clone()],
            },
        })
        .collect();

    let parent_message_id = if api_req.parent_message_id.trim().is_empty() {
        Uuid::new_v4().to_string()
    } else {
        api_req.parent_message_id.trim().to_string()
    };

    types::ChatReq {
        action: "next".to_string(),
        messages,
        conversation_id: api_req.conversation_id.trim().to_string(),
        parent_message_id,
        model: normalize_model(&api_req.model),
        timezone: "Asia/Shanghai".to_string(),
        timezone_offset_min: -480,
        suggestions: vec![],
        supported_encodings: vec![],
        system_hints: vec![],
        history_and_training_disabled: true,
        force_use_sse: true,
        face_use_sse: false,
        force_paragen: false,
        force_paragen_model_slug: String::new(),
        force_rate_limit: false,
        reset_rate_limits: false,
        variant_purpose: "comparison_implicit".to_string(),
        conversation_mode: types::ChatConversationMode {
            kind: "primary_assistant".to_string(),
        },
        websocket_request_id: Uuid::new_v4().to_string(),
        client_contextual_info: types::ClientContextualInfo {
            is_dark_mode: false,
            time_since_loaded: 120,
            page_height: 900,
            page_width: 1400,
            pixel_ratio: 2.0,
            screen_height: 1440,
            screen_width: 2560,
        },
    }
}

fn normalize_model(model: &str) -> String {
    let m = model.trim();
    if m.is_empty() {
        "auto".to_string()
    } else {
        m.to_string()
    }
}
