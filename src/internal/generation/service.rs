use crate::internal::clierrors;
use crate::internal::models::ModelParam;
use serde_json::{Map, Number, Value};
use std::collections::BTreeMap;
use std::time::Duration;

use super::client::{Client, TaskStatus};

pub fn submit(
    api_key: &str,
    endpoint: &str,
    model_id: &str,
    params: BTreeMap<String, Value>,
) -> anyhow::Result<TaskStatus> {
    Client::new(api_key).submit(endpoint, model_id, params)
}

pub fn poll_task(
    api_key: &str,
    generation_endpoint: &str,
    task_id: &str,
    poll_interval: Duration,
    timeout: Duration,
    on_progress: impl FnMut(f64),
) -> anyhow::Result<TaskStatus> {
    Client::new(api_key).poll_task(
        generation_endpoint,
        task_id,
        poll_interval,
        timeout,
        on_progress,
    )
}

pub fn get_task(api_key: &str, task_id: &str) -> anyhow::Result<TaskStatus> {
    Client::new(api_key).get_task(task_id)
}

pub fn parse_params(pairs: &[String]) -> anyhow::Result<BTreeMap<String, String>> {
    let mut out = BTreeMap::new();
    for pair in pairs {
        let Some((key, value)) = pair.split_once('=') else {
            anyhow::bail!("invalid --param {:?}: expected key=value format", pair);
        };
        out.insert(key.to_string(), value.to_string());
    }
    Ok(out)
}

pub fn validate_and_coerce(
    model_id: &str,
    raw: &BTreeMap<String, String>,
    spec_params: &[ModelParam],
) -> anyhow::Result<BTreeMap<String, Value>> {
    let mut out = BTreeMap::new();

    for param in spec_params {
        if is_array_type(&param.param_type) {
            let Some(value) = raw.get(&param.name) else {
                if param.required {
                    return Err(clierrors::err_missing_param(model_id, &param.name).into());
                }
                continue;
            };
            let arr: Vec<Value> = serde_json::from_str(value).map_err(|_| {
                clierrors::err_invalid_param(
                    model_id,
                    &param.name,
                    format!(
                        "expected a JSON array (e.g. '[\"url1\",\"url2\"]' or '[{{\"key\":\"value\"}}]'), got: {value}"
                    ),
                )
            })?;
            out.insert(param.name.clone(), Value::Array(arr));
            continue;
        }

        if param.param_type == "object" && !param.children.is_empty() {
            let prefix = format!("{}.", param.name);
            let child_raw: BTreeMap<String, String> = raw
                .iter()
                .filter_map(|(key, value)| {
                    key.strip_prefix(&prefix)
                        .map(|child_key| (child_key.to_string(), value.clone()))
                })
                .collect();
            if child_raw.is_empty() {
                if param.required {
                    return Err(clierrors::err_missing_param(model_id, &param.name).into());
                }
                continue;
            }
            let nested = validate_and_coerce(model_id, &child_raw, &param.children)?;
            let object = nested.into_iter().collect::<Map<String, Value>>();
            out.insert(param.name.clone(), Value::Object(object));
            continue;
        }

        let Some(value) = raw.get(&param.name) else {
            if param.required {
                return Err(clierrors::err_missing_param(model_id, &param.name).into());
            }
            continue;
        };
        out.insert(param.name.clone(), coerce_value(model_id, param, value)?);
    }

    Ok(out)
}

fn coerce_value(model_id: &str, param: &ModelParam, raw: &str) -> anyhow::Result<Value> {
    if let Some(constraints) = &param.constraints {
        if !constraints.enum_values.is_empty()
            && !constraints.enum_values.iter().any(|allowed| allowed == raw)
        {
            return Err(clierrors::err_invalid_param(
                model_id,
                &param.name,
                format!(
                    "{raw:?} is not allowed. Allowed values: {}",
                    constraints.enum_values.join(", ")
                ),
            )
            .into());
        }
    }

    match param.param_type.as_str() {
        "int" | "integer" => {
            let n: i64 = raw.parse().map_err(|_| {
                clierrors::err_invalid_param(
                    model_id,
                    &param.name,
                    format!("{raw:?} is not a valid integer"),
                )
            })?;
            if let Some(c) = &param.constraints {
                if let Some(min) = c.min {
                    if (n as f64) < min {
                        return Err(clierrors::err_invalid_param(
                            model_id,
                            &param.name,
                            format!("{n} is below minimum {min}"),
                        )
                        .into());
                    }
                }
                if let Some(max) = c.max {
                    if (n as f64) > max {
                        return Err(clierrors::err_invalid_param(
                            model_id,
                            &param.name,
                            format!("{n} exceeds maximum {max}"),
                        )
                        .into());
                    }
                }
            }
            Ok(Value::Number(Number::from(n)))
        }
        "float" | "number" => {
            let f: f64 = raw.parse().map_err(|_| {
                clierrors::err_invalid_param(
                    model_id,
                    &param.name,
                    format!("{raw:?} is not a valid number"),
                )
            })?;
            if let Some(c) = &param.constraints {
                if let Some(min) = c.min {
                    if f < min {
                        return Err(clierrors::err_invalid_param(
                            model_id,
                            &param.name,
                            format!("{f} is below minimum {min}"),
                        )
                        .into());
                    }
                }
                if let Some(max) = c.max {
                    if f > max {
                        return Err(clierrors::err_invalid_param(
                            model_id,
                            &param.name,
                            format!("{f} exceeds maximum {max}"),
                        )
                        .into());
                    }
                }
            }
            Number::from_f64(f).map(Value::Number).ok_or_else(|| {
                clierrors::err_invalid_param(model_id, &param.name, "invalid number").into()
            })
        }
        "boolean" | "bool" => {
            let b: bool = raw.parse().map_err(|_| {
                clierrors::err_invalid_param(
                    model_id,
                    &param.name,
                    format!("{raw:?} is not a valid boolean (use true/false)"),
                )
            })?;
            Ok(Value::Bool(b))
        }
        _ => {
            if let Some(c) = &param.constraints {
                if let Some(min_length) = c.min_length {
                    if raw.len() < min_length {
                        return Err(clierrors::err_invalid_param(
                            model_id,
                            &param.name,
                            format!("value too short (min {min_length} chars)"),
                        )
                        .into());
                    }
                }
                if let Some(max_length) = c.max_length {
                    if raw.len() > max_length {
                        return Err(clierrors::err_invalid_param(
                            model_id,
                            &param.name,
                            format!("value too long (max {max_length} chars)"),
                        )
                        .into());
                    }
                }
            }
            Ok(Value::String(raw.to_string()))
        }
    }
}

fn is_array_type(param_type: &str) -> bool {
    param_type == "array" || param_type.starts_with("array[") || param_type.starts_with("array\\[")
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::internal::models::client::ParamConstraints;
    use crate::internal::models::ModelParam;

    fn param(name: &str, param_type: &str, required: bool) -> ModelParam {
        ModelParam {
            name: name.to_string(),
            param_type: param_type.to_string(),
            required,
            ..serde_json::from_value(serde_json::json!({})).unwrap()
        }
    }

    #[test]
    fn parse_params_accepts_values_with_equals_and_rejects_invalid() {
        let raw = parse_params(&["prompt=a=b".to_string()]).unwrap();
        assert_eq!(raw["prompt"], "a=b");
        assert!(parse_params(&["prompt".to_string()]).is_err());
    }

    #[test]
    fn validate_and_coerce_covers_scalar_array_object_and_errors() {
        let mut int_param = param("count", "int", true);
        int_param.constraints = Some(ParamConstraints {
            min: Some(1.0),
            max: Some(3.0),
            ..ParamConstraints::default()
        });
        let mut mode_param = param("mode", "string", true);
        mode_param.constraints = Some(ParamConstraints {
            enum_values: vec!["fast".to_string(), "pro".to_string()],
            ..ParamConstraints::default()
        });
        let mut prompt_param = param("prompt", "string", true);
        prompt_param.constraints = Some(ParamConstraints {
            min_length: Some(2),
            max_length: Some(5),
            ..ParamConstraints::default()
        });
        let spec = vec![
            int_param,
            param("ratio", "float", true),
            param("enabled", "boolean", true),
            param("items", "array[string]", true),
            ModelParam {
                name: "camera".to_string(),
                param_type: "object".to_string(),
                required: true,
                children: vec![param("speed", "integer", true)],
                ..serde_json::from_value(serde_json::json!({})).unwrap()
            },
            mode_param,
            prompt_param,
        ];
        let raw = BTreeMap::from([
            ("count".to_string(), "2".to_string()),
            ("ratio".to_string(), "1.5".to_string()),
            ("enabled".to_string(), "true".to_string()),
            ("items".to_string(), r#"["a","b"]"#.to_string()),
            ("camera.speed".to_string(), "3".to_string()),
            ("mode".to_string(), "fast".to_string()),
            ("prompt".to_string(), "cat".to_string()),
        ]);

        let out = validate_and_coerce("m", &raw, &spec).unwrap();
        assert_eq!(out["count"], 2);
        assert_eq!(out["ratio"], 1.5);
        assert_eq!(out["enabled"], true);
        assert_eq!(out["items"][0], "a");
        assert_eq!(out["camera"]["speed"], 3);

        let err =
            validate_and_coerce("m", &BTreeMap::new(), &[param("p", "string", true)]).unwrap_err();
        assert!(err.to_string().contains("missing required parameter"));

        let mut raw = BTreeMap::new();
        raw.insert("count".to_string(), "9".to_string());
        let err = validate_and_coerce("m", &raw, &[spec[0].clone()]).unwrap_err();
        assert!(err.to_string().contains("exceeds maximum"));

        raw.insert("items".to_string(), "not-array".to_string());
        let err = validate_and_coerce("m", &raw, &[param("items", "array", true)]).unwrap_err();
        assert!(err.to_string().contains("expected a JSON array"));
    }
}
