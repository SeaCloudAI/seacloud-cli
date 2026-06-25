package cmd

import "net/http"

func writeLLMChatContract(w http.ResponseWriter) {
	_, _ = w.Write([]byte(`{
		"status":{"code":200,"message":"ok"},
		"data":{
			"schema_version":"model-contract.v1",
			"revision":"local-1",
			"model_id":"gpt_4o_mini",
			"protocol":"llm_chat_completions",
			"body_mode":"openai_chat_json",
			"endpoints":{"chat_completions":{"method":"POST","path":"/v1/chat/completions"}},
			"input_schema":{
				"type":"object",
				"required":["messages"],
				"additionalProperties":false,
				"properties":{
					"messages":{"type":"array"},
					"stream":{"type":"boolean"},
					"temperature":{"type":"number"}
				}
			}
		}
	}`))
}

func writeLLMResponsesContract(w http.ResponseWriter) {
	_, _ = w.Write([]byte(`{
		"status":{"code":200,"message":"ok"},
		"data":{
			"schema_version":"model-contract.v1",
			"revision":"local-1",
			"model_id":"gpt_5_mini",
			"protocol":"llm_responses",
			"body_mode":"openai_responses_json",
			"endpoints":{"responses":{"method":"POST","path":"/v1/responses"}},
			"input_schema":{
				"type":"object",
				"required":["input"],
				"additionalProperties":false,
				"properties":{
					"input":{"type":"string"},
					"stream":{"type":"boolean"}
				}
			}
		}
	}`))
}
