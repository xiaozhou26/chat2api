use base64::{engine::general_purpose::STANDARD as BASE64, Engine};
use rand::Rng;
use regex::Regex;
use serde::{Deserialize, Serialize};
use serde_json::Value;
use sha3::{Digest, Sha3_512};
use std::sync::LazyLock;

const DEFAULT_POW_SCRIPT: &str = "https://chatgpt.com/backend-api/sentinel/sdk.js";
const MAX_ATTEMPTS: usize = 500000;

static CORES: &[i32] = &[8, 16, 24, 32];
static SCREEN_VALUES: &[i32] = &[3000, 4000, 5000];
static DOCUMENT_KEYS: &[&str] = &[
    "_reactListeningo743lnnpvdg",
    "location",
];
static NAVIGATOR_KEYS: &[&str] = &[
    "registerProtocolHandler−function registerProtocolHandler() { [native code] }",
    "storage−[object StorageManager]",
    "locks−[object LockManager]",
    "appCodeName−Mozilla",
    "permissions−[object Permissions]",
    "share−function share() { [native code] }",
    "webdriver−false",
    "managed−[object NavigatorManagedData]",
    "canShare−function canShare() { [native code] }",
    "vendor−Google Inc.",
    "mediaDevices−[object MediaDevices]",
    "vibrate−function vibrate() { [native code] }",
    "storageBuckets−[object StorageBucketManager]",
    "mediaCapabilities−[object MediaCapabilities]",
    "cookieEnabled−true",
    "virtualKeyboard−[object VirtualKeyboard]",
    "product−Gecko",
    "presentation−[object Presentation]",
    "onLine−true",
    "mimeTypes−[object MimeTypeArray]",
    "credentials−[object CredentialsContainer]",
    "serviceWorker−[object ServiceWorkerContainer]",
    "keyboard−[object Keyboard]",
    "gpu−[object GPU]",
    "doNotTrack",
    "serial−[object Serial]",
    "pdfViewerEnabled−true",
    "language−zh-CN",
    "geolocation−[object Geolocation]",
    "userAgentData−[object NavigatorUAData]",
    "getUserMedia−function getUserMedia() { [native code] }",
    "sendBeacon−function sendBeacon() { [native code] }",
    "hardwareConcurrency−32",
    "windowControlsOverlay−[object WindowControlsOverlay]",
];
static WINDOW_KEYS: &[&str] = &[
    "0", "window", "self", "document", "name", "location", "customElements",
    "history", "navigation", "innerWidth", "innerHeight", "scrollX", "scrollY",
    "visualViewport", "screenX", "screenY", "outerWidth", "outerHeight",
    "devicePixelRatio", "screen", "chrome", "navigator", "onresize",
    "performance", "crypto", "indexedDB", "sessionStorage", "localStorage",
    "scheduler", "alert", "atob", "btoa", "fetch", "matchMedia", "postMessage",
    "queueMicrotask", "requestAnimationFrame", "setInterval", "setTimeout",
    "caches", "__NEXT_DATA__", "__BUILD_MANIFEST", "__NEXT_PRELOADREADY",
];

static SCRIPT_SRC_RE: LazyLock<Regex> =
    LazyLock::new(|| Regex::new(r#"<script\b[^>]*\bsrc=["']([^"']+)["']"#).unwrap());
static DATA_BUILD_RE: LazyLock<Regex> =
    LazyLock::new(|| Regex::new(r#"(?:c/[^/]*/_|<html[^>]*data-build=["']([^"']*)["'])"#).unwrap());

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct ProofWork {
    #[serde(default)]
    pub difficulty: String,
    #[serde(default)]
    pub required: bool,
    #[serde(default)]
    pub seed: String,
    #[serde(skip)]
    pub ospt: String,
}

#[derive(Debug, Clone, Default)]
pub struct Resources {
    pub script_sources: Vec<String>,
    pub data_build: String,
}

pub fn parse_resources(html: &str) -> Resources {
    let mut resources = Resources::default();

    for cap in SCRIPT_SRC_RE.captures_iter(html) {
        resources.script_sources.push(cap[1].to_string());
        if resources.data_build.is_empty() {
            if let Some(build) = Regex::new(r"c/[^/]*/_")
                .ok()
                .and_then(|re| re.find(&cap[1]).map(|m| m.as_str().to_string()))
            {
                resources.data_build = build;
            }
        }
    }

    if resources.script_sources.is_empty() {
        resources.script_sources.push(DEFAULT_POW_SCRIPT.to_string());
    }

    if resources.data_build.is_empty() {
        for cap in DATA_BUILD_RE.captures_iter(html) {
            if let Some(m) = cap.get(1) {
                if !m.as_str().is_empty() {
                    resources.data_build = m.as_str().to_string();
                    break;
                }
            }
            if !cap[0].is_empty() && cap[0].starts_with("c/") {
                resources.data_build = cap[0].to_string();
                break;
            }
        }
    }

    resources
}

pub fn calc_proof_token(seed: &str, difficulty: &str, user_agent: &str, resources: Option<&Resources>) -> String {
    let res = resources.cloned().unwrap_or_else(|| Resources {
        script_sources: vec![DEFAULT_POW_SCRIPT.to_string()],
        data_build: String::new(),
    });
    if let Some(answer) = generate(seed, difficulty, &build_config(user_agent, &res)) {
        format!("gAAAAAB{}", answer)
    } else {
        String::new()
    }
}

pub fn legacy_requirements_token(user_agent: &str, resources: Option<&Resources>) -> String {
    let mut rng = rand::thread_rng();
    let seed: f64 = rng.gen::<f64>();
    let res = resources.cloned().unwrap_or_else(|| Resources {
        script_sources: vec![DEFAULT_POW_SCRIPT.to_string()],
        data_build: String::new(),
    });
    let answer = generate(&seed.to_string(), "0fffff", &build_config(user_agent, &res))
        .unwrap_or_default();
    format!("gAAAAAC{}", answer)
}

fn build_config(user_agent: &str, resources: &Resources) -> Vec<Value> {
    let mut rng = rand::thread_rng();
    let script_sources = if resources.script_sources.is_empty() {
        vec![DEFAULT_POW_SCRIPT.to_string()]
    } else {
        resources.script_sources.clone()
    };

    let now = chrono::Utc::now();
    let perf_ms = (now.timestamp_nanos_opt().unwrap_or(0) % 1_000_000_000) as f64 / 1_000_000.0;

    vec![
        Value::Number(serde_json::Number::from(SCREEN_VALUES[rng.gen_range(0..SCREEN_VALUES.len())])),
        Value::String(legacy_parse_time()),
        Value::Number(serde_json::Number::from(4294705152i64)),
        Value::Number(serde_json::Number::from(0)),
        Value::String(user_agent.to_string()),
        Value::String(script_sources[rng.gen_range(0..script_sources.len())].clone()),
        Value::String(resources.data_build.clone()),
        Value::String("en-US".to_string()),
        Value::String("en-US,es-US,en,es".to_string()),
        Value::Number(serde_json::Number::from(0)),
        Value::String(NAVIGATOR_KEYS[rng.gen_range(0..NAVIGATOR_KEYS.len())].to_string()),
        Value::String(DOCUMENT_KEYS[rng.gen_range(0..DOCUMENT_KEYS.len())].to_string()),
        Value::String(WINDOW_KEYS[rng.gen_range(0..WINDOW_KEYS.len())].to_string()),
        serde_json::Number::from_f64(perf_ms).map(Value::Number).unwrap_or(Value::Null),
        Value::String(uuid::Uuid::new_v4().to_string()),
        Value::String(String::new()),
        Value::Number(serde_json::Number::from(CORES[rng.gen_range(0..CORES.len())])),
        serde_json::Number::from_f64(now.timestamp_millis() as f64 - perf_ms)
            .map(Value::Number)
            .unwrap_or(Value::Null),
    ]
}

fn legacy_parse_time() -> String {
    let now = chrono::Utc::now();
    let est = chrono::FixedOffset::west_opt(5 * 3600).unwrap();
    let est_time = now.with_timezone(&est);
    format!(
        "{} GMT-0500 (Eastern Standard Time)",
        est_time.format("%a %b %d %Y %H:%M:%S")
    )
}

fn generate(seed: &str, difficulty: &str, config: &[Value]) -> Option<String> {
    let target = hex::decode(difficulty).ok()?;
    if target.is_empty() {
        return None;
    }
    let diff_len = difficulty.len() / 2;
    let seed_bytes = seed.as_bytes();

    let static1 = must_json_prefix(&config[..3]);
    let static2 = must_json_middle(&config[4..9]);
    let static3 = must_json_suffix(&config[10..]);

    let mut hasher = Sha3_512::new();

    for i in 0..MAX_ATTEMPTS {
        let mut final_json = Vec::with_capacity(512);
        final_json.extend_from_slice(&static1);
        final_json.extend_from_slice(format!("{}", i).as_bytes());
        final_json.extend_from_slice(&static2);
        final_json.extend_from_slice(format!("{}", i >> 1).as_bytes());
        final_json.extend_from_slice(&static3);

        let encoded = BASE64.encode(&final_json);

        hasher.update(seed_bytes);
        hasher.update(encoded.as_bytes());
        let digest = hasher.finalize_reset();

        if &digest[..diff_len] <= &target[..diff_len] {
            return Some(encoded);
        }
    }

    None
}

fn must_json_prefix(values: &[Value]) -> Vec<u8> {
    let mut b = serde_json::to_vec(values).unwrap_or_default();
    if !b.is_empty() {
        b.pop(); // remove trailing ']'
        b.push(b',');
    }
    b
}

fn must_json_middle(values: &[Value]) -> Vec<u8> {
    let b = serde_json::to_vec(values).unwrap_or_default();
    let mut result = vec![b','];
    if b.len() > 2 {
        result.extend_from_slice(&b[1..b.len() - 1]);
    }
    result.push(b',');
    result
}

fn must_json_suffix(values: &[Value]) -> Vec<u8> {
    let b = serde_json::to_vec(values).unwrap_or_default();
    let mut result = vec![b','];
    if b.len() > 1 {
        result.extend_from_slice(&b[1..]);
    }
    result
}
