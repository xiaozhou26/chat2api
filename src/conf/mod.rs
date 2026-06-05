use serde::Deserialize;
use std::env;
use std::fs;
use std::path::{Path, PathBuf};
use std::sync::{Arc, RwLock};
use std::time::Duration;

#[derive(Debug, Clone, Deserialize)]
pub struct AppConfig {
    #[serde(default = "default_log_level")]
    pub log_level: String,
    #[serde(default = "default_log_path")]
    pub log_path: String,
    #[serde(default = "default_log_file")]
    pub log_file: String,
    #[serde(default = "default_bind")]
    pub bind: String,
    #[serde(default = "default_port")]
    pub port: u16,
    #[serde(default)]
    pub auth: AuthConfig,
    #[serde(default)]
    pub proxy: String,
    #[serde(default = "default_chatgpt_base_url")]
    pub chatgpt_base_url: String,
    #[serde(default)]
    pub chatgpts: Vec<ChatGPTAccount>,
}

#[derive(Debug, Clone, Deserialize)]
pub struct AuthConfig {
    #[serde(default)]
    pub access_tokens: Vec<String>,
}

impl Default for AuthConfig {
    fn default() -> Self {
        Self {
            access_tokens: Vec::new(),
        }
    }
}

#[derive(Debug, Clone, Deserialize)]
pub struct ChatGPTAccount {
    #[serde(default)]
    pub id_token: String,
    #[serde(default)]
    pub access_token: String,
    #[serde(default)]
    pub refresh_token: String,
    #[serde(default)]
    pub account_id: String,
    #[serde(default)]
    pub last_refresh: String,
    #[serde(default)]
    pub email: String,
    #[serde(default)]
    #[serde(rename = "type")]
    pub account_type: String,
    #[serde(default)]
    pub expired: String,
    #[serde(default)]
    pub proxy: String,
}

fn default_log_level() -> String { "debug".into() }
fn default_log_path() -> String { "logs".into() }
fn default_log_file() -> String { "app.dev.log".into() }
fn default_bind() -> String { "0.0.0.0".into() }
fn default_port() -> u16 { 3040 }
fn default_chatgpt_base_url() -> String { "https://chatgpt.com".into() }

impl Default for AppConfig {
    fn default() -> Self {
        Self {
            log_level: default_log_level(),
            log_path: default_log_path(),
            log_file: default_log_file(),
            bind: default_bind(),
            port: default_port(),
            auth: AuthConfig::default(),
            proxy: String::new(),
            chatgpt_base_url: default_chatgpt_base_url(),
            chatgpts: Vec::new(),
        }
    }
}

pub fn config_path() -> PathBuf {
    let env = env::var("ENV").unwrap_or_else(|_| "dev".to_string());
    let cwd = env::current_dir().unwrap_or_else(|_| PathBuf::from("."));
    cwd.join("conf").join(format!("app.{}.yaml", env))
}

pub fn load_config() -> AppConfig {
    let path = config_path();
    load_config_from(&path)
}

pub fn load_config_from(path: &Path) -> AppConfig {
    match fs::read_to_string(path) {
        Ok(content) => {
            let mut config: AppConfig = serde_yaml::from_str(&content).unwrap_or_default();
            normalize_config(&mut config, path);
            config
        }
        Err(e) => {
            tracing::error!("load config failed from {:?}: {}, use default config", path, e);
            AppConfig::default()
        }
    }
}

fn normalize_config(config: &mut AppConfig, path: &Path) {
    // 归一化 auth tokens
    config.auth.access_tokens = config
        .auth
        .access_tokens
        .iter()
        .map(|t| normalize_auth_token(t))
        .filter(|t| !t.is_empty())
        .collect();

    // 如果 auth tokens 为空，生成一个随机 sk- token 并写回配置文件
    if config.auth.access_tokens.is_empty() {
        let token = generate_auth_token();
        config.auth.access_tokens.push(token.clone());
        let _ = save_auth_tokens(path, &config.auth.access_tokens);
    }

    tracing::info!("current auth: {}", config.auth.access_tokens.join(", "));
}

pub fn normalize_auth_token(token: &str) -> String {
    let t = token.trim().trim_start_matches("Bearer ").trim().to_string();
    t
}

fn generate_auth_token() -> String {
    use rand::Rng;
    let mut rng = rand::thread_rng();
    let mut buf = [0u8; 24];
    rng.fill(&mut buf);
    format!("sk-{}", hex::encode(buf))
}

fn save_auth_tokens(path: &Path, tokens: &[String]) -> Result<(), Box<dyn std::error::Error>> {
    let content = fs::read_to_string(path)?;
    let mut doc: serde_yaml::Value = serde_yaml::from_str(&content)?;

    if let Some(mapping) = doc.as_mapping_mut() {
        let auth_key = serde_yaml::Value::String("auth".into());
        if let Some(auth_val) = mapping.get_mut(&auth_key) {
            if let Some(auth_map) = auth_val.as_mapping_mut() {
                let tokens_key = serde_yaml::Value::String("access_tokens".into());
                let tokens_val: serde_yaml::Value = serde_yaml::Value::Sequence(
                    tokens
                        .iter()
                        .filter(|t| !t.is_empty())
                        .map(|t| serde_yaml::Value::String(t.clone()))
                        .collect(),
                );
                auth_map.insert(tokens_key, tokens_val);
            }
        }
    }

    let new_content = serde_yaml::to_string(&doc)?;
    fs::write(path, new_content)?;
    Ok(())
}

/// 配置文件热加载监视器
pub struct ConfigWatcher {
    config: Arc<RwLock<AppConfig>>,
    path: PathBuf,
}

impl ConfigWatcher {
    pub fn new(config: Arc<RwLock<AppConfig>>) -> Self {
        Self {
            config,
            path: config_path(),
        }
    }

    pub fn start(self) -> tokio::task::JoinHandle<()> {
        let path = self.path.clone();
        let config = self.config.clone();

        tokio::spawn(async move {
            let (tx, mut rx) = tokio::sync::mpsc::channel(1);
            let path_clone = path.clone();

            // 在后台线程中监视文件变化
            std::thread::spawn(move || {
                use notify::{EventKind, Watcher};
                let mut watcher = notify::recommended_watcher(move |res: Result<notify::Event, notify::Error>| {
                    if let Ok(event) = res {
                        if matches!(event.kind, EventKind::Modify(_)) {
                            let _ = tx.blocking_send(());
                        }
                    }
                })
                .expect("failed to create file watcher");

                use notify::RecursiveMode;
                let _ = watcher.watch(&path_clone, RecursiveMode::NonRecursive);
                // 保持 watcher 存活
                std::thread::sleep(Duration::from_secs(u64::MAX));
            });

            while rx.recv().await.is_some() {
                tracing::info!("config file changed, reloading...");
                let new_config = load_config_from(&path);
                if let Ok(mut guard) = config.write() {
                    *guard = new_config;
                }
                // 重新初始化账号池
                crate::acc_token_pool::reinit(&config);
            }
        })
    }
}
