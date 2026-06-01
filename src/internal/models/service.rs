use crate::internal::clierrors;

use super::aliases::{
    display_model_id, preferred_model_id, resolve_model_id, rewrite_model_id_text,
};
use super::client::{Client, ListParams, ModelSpec, ModelsListResponse};

pub fn list(params: ListParams) -> anyhow::Result<ModelsListResponse> {
    let mut result = Client::new()
        .list(params)
        .map_err(clierrors::err_fetch_models)?;
    for model in &mut result.models {
        model.id = display_model_id(&model.id);
    }
    Ok(result)
}

pub fn get_spec(model_id: &str) -> anyhow::Result<ModelSpec> {
    let resolved_model_id = resolve_model_id(model_id);
    let mut spec = Client::new().get_spec(&resolved_model_id).map_err(|err| {
        if is_not_found(&err) {
            clierrors::err_model_not_found(model_id)
        } else {
            clierrors::err_fetch_model_spec(model_id, err)
        }
    })?;

    let backend_model_id = if spec.model_id.is_empty() {
        resolved_model_id
    } else {
        spec.model_id.clone()
    };
    let display_model_id = preferred_model_id(model_id, &backend_model_id);
    spec.model_id = display_model_id.clone();
    spec.agent_prompt =
        rewrite_model_id_text(&spec.agent_prompt, &backend_model_id, &display_model_id);
    Ok(spec)
}

fn is_not_found(err: &anyhow::Error) -> bool {
    let s = err.to_string();
    s.len() >= 10 && &s[..10] == "status 404"
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::internal::test_support::{TestResponse, TestServer};
    use serial_test::serial;
    use std::env;

    #[test]
    #[serial]
    fn list_and_get_spec_apply_display_aliases() {
        env::remove_var("SEACLOUD_MODELS_URL");
        let server = TestServer::new(|req| {
            if req.path == "/api/v1/skill/models" {
                return TestResponse::json(
                    200,
                    r#"{"status":{"code":200,"message":"ok"},"data":{"models":[{"id":"kirin_v2_6_i2v","name":"Kling","type":"video"}],"total":1,"page":1,"page_size":20,"total_pages":1}}"#,
                );
            }
            assert_eq!(req.path, "/api/v1/skill/models/kirin_v3_t2v/spec");
            TestResponse::json(
                200,
                r#"{"status":{"code":200,"message":"ok"},"data":{"model_id":"kirin_v3_t2v","name":"Kling","vendor":"kling","type":"video","api":{"endpoint":"https://example.com/model","method":"POST","headers":{}},"parameters":[],"agent_prompt":"submit kirin_v3_t2v"}}"#,
            )
        });
        env::set_var("SEACLOUD_MODELS_URL", server.url());

        let listed = list(ListParams::default()).unwrap();
        assert_eq!(listed.models[0].id, "kling_v2_6_i2v");

        let spec = get_spec("kling_v3_t2v").unwrap();
        assert_eq!(spec.model_id, "kling_v3_t2v");
        assert_eq!(spec.agent_prompt, "submit kling_v3_t2v");
    }
}
