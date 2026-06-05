pub mod conf;
pub mod logx;
pub mod acc_token_pool;
pub mod proof_work;
pub mod turnstile;
pub mod chat_backend;
pub mod types;
pub mod router;
pub mod middleware;
pub mod service;

use std::sync::Arc;
use tokio::signal;
use tokio::sync::broadcast;

pub async fn run() {
    // 初始化日志
    logx::init();

    tracing::info!("application process in PID: {}", std::process::id());

    // 加载配置
    let config = conf::load_config();
    let config = Arc::new(std::sync::RwLock::new(config));

    // 初始化账号池
    {
        let guard = config.read().unwrap();
        acc_token_pool::init(&guard);
    }

    // 启动配置热加载
    let config_watcher = conf::ConfigWatcher::new(Arc::clone(&config));
    let _watcher_guard = config_watcher.start();

    // 启动 HTTP 服务
    let (tx, rx) = broadcast::channel::<()>(1);
    let server = router::start(config.clone(), rx);

    // 等待退出信号
    match signal::ctrl_c().await {
        Ok(()) => tracing::info!("received shutdown signal"),
        Err(e) => tracing::error!("failed to listen for ctrl-c: {}", e),
    }

    // 通知关闭
    let _ = tx.send(());
    let _ = server.await;
    tracing::info!("server shutdown complete");
}

fn main() {
    tokio::runtime::Builder::new_multi_thread()
        .enable_all()
        .build()
        .unwrap()
        .block_on(run());
}
