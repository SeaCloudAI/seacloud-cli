package llm

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/SeaCloudAI/seacloud-cli/internal/clierrors"
)

func parseChatCompletion(body []byte) (*Result, error) {
	var payload struct {
		ID      string         `json:"id"`
		Model   string         `json:"model"`
		Usage   map[string]any `json:"usage"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("unexpected chat completions response: %s", string(body))
	}
	result := &Result{ID: payload.ID, Model: payload.Model, Usage: payload.Usage}
	if len(payload.Choices) > 0 {
		result.Text = payload.Choices[0].Message.Content
		result.FinishReason = payload.Choices[0].FinishReason
	}
	return result, nil
}

func parseResponsesCompletion(body []byte) (*Result, error) {
	var payload struct {
		ID         string         `json:"id"`
		Model      string         `json:"model"`
		OutputText string         `json:"output_text"`
		Usage      map[string]any `json:"usage"`
		Output     []struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("unexpected responses response: %s", string(body))
	}
	text := payload.OutputText
	if text == "" {
		var parts []string
		for _, item := range payload.Output {
			for _, content := range item.Content {
				if content.Text != "" {
					parts = append(parts, content.Text)
				}
			}
		}
		text = strings.Join(parts, "")
	}
	return &Result{ID: payload.ID, Model: payload.Model, Text: text, Usage: payload.Usage}, nil
}

type streamState struct {
	protocol  string
	result    Result
	text      strings.Builder
	completed bool
}

func (s *streamState) dispatch(event, data string, opts StreamOptions) error {
	trimmed := strings.TrimSpace(data)
	if trimmed == "" {
		return nil
	}
	if trimmed == "[DONE]" {
		s.completed = true
		return nil
	}
	if event == "error" {
		return sseAPIError(data)
	}
	switch s.protocol {
	case ProtocolChatCompletions:
		return s.applyChatChunk(data, opts)
	case ProtocolResponses:
		return s.applyResponsesEvent(event, data, opts)
	default:
		return fmt.Errorf("unsupported LLM protocol: %s", s.protocol)
	}
}

func (s *streamState) applyChatChunk(data string, opts StreamOptions) error {
	var chunk struct {
		ID      string         `json:"id"`
		Model   string         `json:"model"`
		Usage   map[string]any `json:"usage"`
		Choices []struct {
			Delta struct {
				Content string `json:"content"`
			} `json:"delta"`
			FinishReason *string `json:"finish_reason"`
		} `json:"choices"`
	}
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return fmt.Errorf("unexpected chat completions SSE data: %s", data)
	}
	if chunk.ID != "" {
		s.result.ID = chunk.ID
	}
	if chunk.Model != "" {
		s.result.Model = chunk.Model
	}
	if chunk.Usage != nil {
		s.result.Usage = chunk.Usage
	}
	for _, choice := range chunk.Choices {
		if choice.Delta.Content != "" {
			if err := s.appendText(choice.Delta.Content, opts); err != nil {
				return err
			}
		}
		if choice.FinishReason != nil && *choice.FinishReason != "" {
			s.result.FinishReason = *choice.FinishReason
			s.completed = true
		}
	}
	return nil
}

func (s *streamState) applyResponsesEvent(event, data string, opts StreamOptions) error {
	var msg struct {
		Type     string         `json:"type"`
		Delta    string         `json:"delta"`
		Usage    map[string]any `json:"usage"`
		Response struct {
			ID         string         `json:"id"`
			Model      string         `json:"model"`
			OutputText string         `json:"output_text"`
			Usage      map[string]any `json:"usage"`
		} `json:"response"`
		Error any `json:"error"`
	}
	if err := json.Unmarshal([]byte(data), &msg); err != nil {
		return fmt.Errorf("unexpected responses SSE data: %s", data)
	}
	eventType := msg.Type
	if eventType == "" {
		eventType = event
	}
	if msg.Error != nil || eventType == "response.failed" {
		return sseAPIError(data)
	}
	switch eventType {
	case "response.output_text.delta":
		if msg.Delta != "" {
			return s.appendText(msg.Delta, opts)
		}
	case "response.completed":
		s.completed = true
		if msg.Response.ID != "" {
			s.result.ID = msg.Response.ID
		}
		if msg.Response.Model != "" {
			s.result.Model = msg.Response.Model
		}
		if msg.Response.OutputText != "" && s.text.Len() == 0 {
			s.text.WriteString(msg.Response.OutputText)
		}
		if msg.Response.Usage != nil {
			s.result.Usage = msg.Response.Usage
		}
	default:
		if msg.Usage != nil {
			s.result.Usage = msg.Usage
		}
	}
	return nil
}

func (s *streamState) appendText(text string, opts StreamOptions) error {
	s.text.WriteString(text)
	if opts.OnText != nil {
		return opts.OnText(text)
	}
	return nil
}

func (s *streamState) finish() *Result {
	s.result.Text = s.text.String()
	return &s.result
}

func sseAPIError(data string) error {
	apiErr := clierrors.NewAPIError(http.StatusInternalServerError, []byte(data))
	var payload struct {
		Error any `json:"error"`
	}
	if json.Unmarshal([]byte(data), &payload) == nil && payload.Error != nil && apiErr.Message == "" {
		apiErr.Message = fmt.Sprint(payload.Error)
	}
	return apiErr
}
