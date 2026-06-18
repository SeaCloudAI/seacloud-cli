package contracts

import "errors"

const SupportedSchemaVersion = "model-contract.v1"

var (
	ErrNotFound           = errors.New("model contract not found")
	ErrIncompatibleSchema = errors.New("model contract schema version is incompatible")
)

type Options struct {
	Refresh bool
}

type ModelContract struct {
	SchemaVersion  string            `json:"schema_version"`
	Revision       string            `json:"revision"`
	ModelID        string            `json:"model_id"`
	BackendModelID string            `json:"-"`
	DisplayName    string            `json:"display_name"`
	Family         string            `json:"family"`
	Kind           string            `json:"kind"`
	Protocol       string            `json:"protocol"`
	BodyMode       string            `json:"body_mode"`
	Endpoints      ContractEndpoints `json:"endpoints"`
	InputSchema    InputSchema       `json:"input_schema"`
	Prerequisites  []Prerequisite    `json:"prerequisites,omitempty"`
	ContextInputs  []ContextInput    `json:"context_inputs,omitempty"`
	FieldAliases   []FieldAlias      `json:"field_aliases,omitempty"`
	InputRules     []InputRule       `json:"input_rules,omitempty"`
}

type ContractEndpoints struct {
	Submit Endpoint `json:"submit"`
	Status Endpoint `json:"status"`
	Result Endpoint `json:"result"`
	Cancel Endpoint `json:"cancel"`
}

type Endpoint struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}

type Prerequisite struct {
	Field       string `json:"field"`
	SourceModel string `json:"source_model"`
	ContextKind string `json:"context_kind"`
	SourcePath  string `json:"source_path,omitempty"`
	Notes       string `json:"notes,omitempty"`
}

type ContextInput struct {
	Field       string `json:"field"`
	SourceModel string `json:"source_model"`
	ContextKind string `json:"context_kind"`
	SourcePath  string `json:"source_path,omitempty"`
	Required    bool   `json:"required"`
	Notes       string `json:"notes,omitempty"`
}

type FieldAlias struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Reason string `json:"reason,omitempty"`
}

type InputRule struct {
	Kind     string         `json:"kind"`
	Fields   []string       `json:"fields,omitempty"`
	When     map[string]any `json:"when,omitempty"`
	Required []string       `json:"required,omitempty"`
	Notes    string         `json:"notes,omitempty"`
}

type InputSchema struct {
	Type                 string                 `json:"type"`
	Required             []string               `json:"required,omitempty"`
	AdditionalProperties *bool                  `json:"additionalProperties,omitempty"`
	Properties           map[string]InputSchema `json:"properties,omitempty"`
	Items                *InputSchema           `json:"items,omitempty"`
	Enum                 []any                  `json:"enum,omitempty"`
	Default              any                    `json:"default,omitempty"`
	Minimum              *float64               `json:"minimum,omitempty"`
	Maximum              *float64               `json:"maximum,omitempty"`
	MultipleOf           *float64               `json:"multipleOf,omitempty"`
	MinLength            *int                   `json:"minLength,omitempty"`
	MaxLength            *int                   `json:"maxLength,omitempty"`
	MinItems             *int                   `json:"minItems,omitempty"`
	MaxItems             *int                   `json:"maxItems,omitempty"`
	Pattern              string                 `json:"pattern,omitempty"`
	Format               string                 `json:"format,omitempty"`
}
