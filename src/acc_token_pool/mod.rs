use crate::conf::AppConfig;
use std::sync::atomic::{AtomicI64, AtomicUsize, Ordering};
use std::sync::RwLock;

/// 账号池中的单个账号
#[derive(Debug)]
pub struct AccessToken {
    pub token: String,
    pub expires_at: i64,
    pub proxy: String,
    pub can_use_at: AtomicI64,
}

impl Clone for AccessToken {
    fn clone(&self) -> Self {
        Self {
            token: self.token.clone(),
            expires_at: self.expires_at,
            proxy: self.proxy.clone(),
            can_use_at: AtomicI64::new(self.can_use_at.load(Ordering::Relaxed)),
        }
    }
}

impl AccessToken {
    pub fn new(token: String, proxy: String) -> Self {
        let expires_at = chrono::Utc::now()
            .checked_add_signed(chrono::Duration::hours(24))
            .map(|t| t.timestamp())
            .unwrap_or(0);
        Self {
            token,
            expires_at,
            proxy,
            can_use_at: AtomicI64::new(0),
        }
    }

    pub fn can_use(&self) -> bool {
        let now = chrono::Utc::now().timestamp();
        self.can_use_at.load(Ordering::Relaxed) <= now && self.expires_at > now
    }
}

/// 账号池
pub struct AccessTokenPool {
    tokens: RwLock<Vec<AccessToken>>,
    index: AtomicUsize,
}

impl AccessTokenPool {
    fn new() -> Self {
        Self {
            tokens: RwLock::new(Vec::new()),
            index: AtomicUsize::new(0),
        }
    }

    pub fn size(&self) -> usize {
        self.tokens.read().unwrap().len()
    }

    pub fn is_empty(&self) -> bool {
        self.size() == 0
    }

    pub fn can_use_size(&self) -> usize {
        self.tokens
            .read()
            .unwrap()
            .iter()
            .filter(|t| t.can_use())
            .count()
    }

    pub fn get_access_token(&self) -> Option<AccessToken> {
        let tokens = self.tokens.read().unwrap();
        if tokens.is_empty() {
            return None;
        }
        let len = tokens.len();
        drop(tokens);

        for _ in 0..len {
            let tokens = self.tokens.read().unwrap();
            let idx = self.index.fetch_add(1, Ordering::Relaxed) % tokens.len();
            if tokens[idx].can_use() {
                return Some(tokens[idx].clone());
            }
        }
        None
    }

    pub fn set_can_use_at(&self, token_str: &str, can_use_at: i64) {
        let tokens = self.tokens.read().unwrap();
        for t in tokens.iter() {
            if t.token == token_str {
                t.can_use_at.store(can_use_at, Ordering::Relaxed);
                break;
            }
        }
    }

    fn reset(&self) {
        let mut tokens = self.tokens.write().unwrap();
        tokens.clear();
        self.index.store(0, Ordering::Relaxed);
    }

    fn add(&self, token: AccessToken) {
        self.tokens.write().unwrap().push(token);
    }
}

use std::sync::OnceLock;

static POOL: OnceLock<AccessTokenPool> = OnceLock::new();

pub fn pool() -> &'static AccessTokenPool {
    POOL.get_or_init(AccessTokenPool::new)
}

pub fn init(config: &AppConfig) {
    let p = pool();
    p.reset();
    for account in &config.chatgpts {
        let token = account.access_token.trim().trim_start_matches("Bearer ").trim();
        if token.is_empty() {
            continue;
        }
        p.add(AccessToken::new(
            format!("Bearer {}", token),
            account.proxy.trim().to_string(),
        ));
    }
}

pub fn reinit(config: &std::sync::Arc<RwLock<AppConfig>>) {
    if let Ok(guard) = config.read() {
        init(&guard);
    }
}
