use base64::{engine::general_purpose::STANDARD as BASE64, Engine};
use rand::Rng;
use serde_json::Value;
use std::collections::BTreeMap;
use std::time::Instant;

/// Turnstile 求解器
pub fn solve(dx: &str, p: &str) -> String {
    let decoded = match BASE64.decode(dx) {
        Ok(d) => d,
        Err(_) => return String::new(),
    };

    let decrypted = xor_string(&String::from_utf8_lossy(&decoded), p);
    let token_list: Vec<Value> = match serde_json::from_str(&decrypted) {
        Ok(v) => v,
        Err(_) => return String::new(),
    };

    let mut process_map: BTreeMap<i64, Value> = BTreeMap::new();
    let start_time = Instant::now();
    let mut result = String::new();
    let p_owned = p.to_string();

    // 预定义操作码
    process_map.insert(1, serde_json::json!("op_xor"));
    process_map.insert(2, serde_json::json!("op_set"));
    process_map.insert(3, serde_json::json!("op_result"));
    process_map.insert(5, serde_json::json!("op_append"));
    process_map.insert(6, serde_json::json!("op_dot_access"));
    process_map.insert(7, serde_json::json!("op_call"));
    process_map.insert(8, serde_json::json!("op_copy"));
    process_map.insert(9, serde_json::json!(token_list));
    process_map.insert(10, serde_json::json!("window"));
    process_map.insert(14, serde_json::json!("op_json_parse"));
    process_map.insert(15, serde_json::json!("op_json_stringify"));
    process_map.insert(16, serde_json::json!(p_owned));
    process_map.insert(17, serde_json::json!("op_method_call"));
    process_map.insert(18, serde_json::json!("op_b64_decode"));
    process_map.insert(19, serde_json::json!("op_b64_encode"));
    process_map.insert(20, serde_json::json!("op_branch_eq"));
    process_map.insert(21, serde_json::json!("op_nop"));
    process_map.insert(23, serde_json::json!("op_if_not_null"));
    process_map.insert(24, serde_json::json!("op_dot_join"));

    for token in &token_list {
        let items = match token.as_array() {
            Some(arr) => arr,
            None => continue,
        };
        if items.is_empty() {
            continue;
        }
        let opcode = as_f64(&items[0]) as i64;

        let _ = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| {
            match opcode {
                1 => {
                    // xor
                    let e = as_f64(&items[1]) as i64;
                    let t = as_f64(&items[2]) as i64;
                    let ev = ts_string(process_map.get(&e));
                    let tv = ts_string(process_map.get(&t));
                    process_map.insert(e, serde_json::json!(xor_string(&ev, &tv)));
                }
                2 => {
                    // set
                    let e = as_f64(&items[1]) as i64;
                    process_map.insert(e, items.get(2).cloned().unwrap_or(Value::Null));
                }
                3 => {
                    // result
                    result = BASE64.encode(ts_string(items.get(2).or(process_map.get(&(as_f64(&items[1]) as i64)))));
                }
                5 => {
                    // append / concat
                    let e = as_f64(&items[1]) as i64;
                    let t = as_f64(&items[2]) as i64;
                    let current = process_map.get(&e).cloned().unwrap_or(Value::Null);
                    let incoming = process_map.get(&t).cloned().unwrap_or(Value::Null);
                    if let Some(arr) = current.as_array() {
                        let mut new_arr = arr.clone();
                        new_arr.push(incoming);
                        process_map.insert(e, Value::Array(new_arr));
                    } else if current.is_string() || current.is_number() || incoming.is_string() || incoming.is_number() {
                        let cv = ts_string(Some(&current));
                        let iv = ts_string(Some(&incoming));
                        process_map.insert(e, serde_json::json!(format!("{}{}", cv, iv)));
                    } else {
                        process_map.insert(e, serde_json::json!("NaN"));
                    }
                }
                6 => {
                    // dot access
                    let e = as_f64(&items[1]) as i64;
                    let t = as_f64(&items[2]) as i64;
                    let n = as_f64(&items[3]) as i64;
                    let tv = ts_string(process_map.get(&t));
                    let nv = ts_string(process_map.get(&n));
                    let joined = format!("{}.{}", tv, nv);
                    if joined == "window.document.location" {
                        process_map.insert(e, serde_json::json!("https://chatgpt.com/"));
                    } else {
                        process_map.insert(e, serde_json::json!(joined));
                    }
                }
                7 => {
                    // call
                    let target = ts_string(process_map.get(&(as_f64(&items[1]) as i64)));
                    let call_args: Vec<Value> = items[2..]
                        .iter()
                        .map(|a| process_map.get(&(as_f64(a) as i64)).cloned().unwrap_or(Value::Null))
                        .collect();

                    if target == "window.Reflect.set" && call_args.len() >= 3 {
                        // Reflect.set - 忽略，简化处理
                    }
                    // 其他 call 操作在 method_call 中处理
                }
                8 => {
                    // copy
                    let e = as_f64(&items[1]) as i64;
                    let t = as_f64(&items[2]) as i64;
                    if let Some(v) = process_map.get(&t).cloned() {
                        process_map.insert(e, v);
                    }
                }
                14 => {
                    // json parse
                    let e = as_f64(&items[1]) as i64;
                    let t = as_f64(&items[2]) as i64;
                    let s = ts_string(process_map.get(&t));
                    if let Ok(v) = serde_json::from_str(&s) {
                        process_map.insert(e, v);
                    }
                }
                15 => {
                    // json stringify
                    let e = as_f64(&items[1]) as i64;
                    let t = as_f64(&items[2]) as i64;
                    if let Some(v) = process_map.get(&t) {
                        process_map.insert(e, serde_json::json!(py_json_dumps(v)));
                    }
                }
                17 => {
                    // method call
                    let e = as_f64(&items[1]) as i64;
                    let target = ts_string(process_map.get(&(as_f64(&items[2]) as i64)));
                    let call_args: Vec<Value> = items[3..]
                        .iter()
                        .map(|a| process_map.get(&(as_f64(a) as i64)).cloned().unwrap_or(Value::Null))
                        .collect();

                    match target.as_str() {
                        "window.performance.now" => {
                            let elapsed = start_time.elapsed().as_nanos() as f64 / 1e6;
                            let mut rng = rand::thread_rng();
                            let val = elapsed + rng.gen::<f64>() / 1e6;
                            process_map.insert(e, serde_json::Number::from_f64(val).map(Value::Number).unwrap_or(Value::Null));
                        }
                        "window.Object.create" => {
                            process_map.insert(e, serde_json::json!({}));
                        }
                        "window.Object.keys" => {
                            if !call_args.is_empty() && ts_string(Some(&call_args[0])) == "window.localStorage" {
                                process_map.insert(e, serde_json::json!([
                                    "STATSIG_LOCAL_STORAGE_INTERNAL_STORE_V4",
                                    "STATSIG_LOCAL_STORAGE_STABLE_ID",
                                    "client-correlated-secret",
                                    "oai/apps/capExpiresAt",
                                    "oai-did",
                                    "STATSIG_LOCAL_STORAGE_LOGGING_REQUEST",
                                    "UiState.isNavigationCollapsed.1",
                                ]));
                            }
                        }
                        "window.Math.random" => {
                            let mut rng = rand::thread_rng();
                            let val: f64 = rng.gen();
                            process_map.insert(e, serde_json::Number::from_f64(val).map(Value::Number).unwrap_or(Value::Null));
                        }
                        _ => {}
                    }
                }
                18 => {
                    // base64 decode
                    let e = as_f64(&items[1]) as i64;
                    let s = ts_string(process_map.get(&e));
                    if let Ok(decoded) = BASE64.decode(&s) {
                        process_map.insert(e, serde_json::json!(String::from_utf8_lossy(&decoded).to_string()));
                    }
                }
                19 => {
                    // base64 encode
                    let e = as_f64(&items[1]) as i64;
                    let s = ts_string(process_map.get(&e));
                    process_map.insert(e, serde_json::json!(BASE64.encode(s.as_bytes())));
                }
                20 => {
                    // branch eq
                    let e = as_f64(&items[1]) as i64;
                    let t = as_f64(&items[2]) as i64;
                    let _n = as_f64(&items[3]) as i64;
                    let ev = process_map.get(&e);
                    let tv = process_map.get(&t);
                    if ev == tv {
                        // 执行 n 操作码（简化处理）
                    }
                }
                21 => {
                    // nop
                }
                23 => {
                    // if not null
                    let e = as_f64(&items[1]) as i64;
                    if let Some(v) = process_map.get(&e) {
                        if !v.is_null() {
                            // 执行操作（简化处理）
                        }
                    }
                }
                24 => {
                    // dot join
                    let e = as_f64(&items[1]) as i64;
                    let t = as_f64(&items[2]) as i64;
                    let n = as_f64(&items[3]) as i64;
                    let tv = ts_string(process_map.get(&t));
                    let nv = ts_string(process_map.get(&n));
                    process_map.insert(e, serde_json::json!(format!("{}.{}", tv, nv)));
                }
                _ => {}
            }
        }));
    }

    result
}

fn ts_string(v: Option<&Value>) -> String {
    match v {
        None => "undefined".to_string(),
        Some(Value::String(s)) => match s.as_str() {
            "window.Math" => "[object Math]".to_string(),
            "window.Reflect" => "[object Reflect]".to_string(),
            "window.performance" => "[object Performance]".to_string(),
            "window.localStorage" => "[object Storage]".to_string(),
            "window.Object" => "function Object() { [native code] }".to_string(),
            "window.Reflect.set" => "function set() { [native code] }".to_string(),
            "window.performance.now" => "function () { [native code] }".to_string(),
            "window.Object.create" => "function create() { [native code] }".to_string(),
            "window.Object.keys" => "function keys() { [native code] }".to_string(),
            "window.Math.random" => "function random() { [native code] }".to_string(),
            _ => s.clone(),
        },
        Some(Value::Array(arr)) => {
            if arr.iter().all(|i| i.is_string()) {
                arr.iter()
                    .map(|i| ts_string(Some(i)))
                    .collect::<Vec<_>>()
                    .join(",")
            } else {
                "undefined".to_string()
            }
        }
        Some(Value::Number(n)) => {
            if let Some(f) = n.as_f64() {
                if f == f.trunc() {
                    format!("{}", f as i64)
                } else {
                    format!("{}", f)
                }
            } else {
                "0".to_string()
            }
        }
        Some(Value::Bool(b)) => {
            if *b {
                "True".to_string()
            } else {
                "False".to_string()
            }
        }
        _ => "undefined".to_string(),
    }
}

fn xor_string(text: &str, key: &str) -> String {
    if key.is_empty() {
        return text.to_string();
    }
    let key_chars: Vec<char> = key.chars().collect();
    text.chars()
        .enumerate()
        .map(|(i, ch)| {
            let k = key_chars[i % key_chars.len()];
            let result = (ch as u32) ^ (k as u32);
            char::from_u32(result).unwrap_or(ch)
        })
        .collect()
}

fn as_f64(v: &Value) -> f64 {
    match v {
        Value::Number(n) => n.as_f64().unwrap_or(0.0),
        _ => 0.0,
    }
}

fn py_json_dumps(v: &Value) -> String {
    match v {
        Value::Object(map) => {
            let parts: Vec<String> = map
                .iter()
                .map(|(k, v)| format!("{}: {}", serde_json::to_string(k).unwrap_or_default(), py_json_dumps(v)))
                .collect();
            format!("{{{}}}", parts.join(", "))
        }
        Value::Array(arr) => {
            let parts: Vec<String> = arr.iter().map(py_json_dumps).collect();
            format!("[{}]", parts.join(", "))
        }
        _ => serde_json::to_string(v).unwrap_or_default(),
    }
}
