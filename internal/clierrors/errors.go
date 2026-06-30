package clierrors

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// CLIError is an error with a human-readable hint for what to do next.
// Cobra prints err.Error() directly, so the hint appears inline.
type CLIError struct {
	Message string
	Hint    string
}

type APIError struct {
	HTTPStatus int
	StatusCode int
	ErrorCode  int
	Message    string
	Body       string
}

func (e *CLIError) Error() string {
	if e.Hint != "" {
		return e.Message + "\n  Hint: " + e.Hint
	}
	return e.Message
}

func (e *APIError) Error() string {
	msg := e.Message
	if msg == "" {
		msg = e.Body
	}
	return fmt.Sprintf("HTTP %d: %s", e.HTTPStatus, msg)
}

func NewAPIError(httpStatus int, body []byte) *APIError {
	apiErr := &APIError{HTTPStatus: httpStatus, Body: string(body)}
	var payload struct {
		Message   string `json:"message"`
		Error     string `json:"error"`
		ErrorCode int    `json:"error_code"`
		Status    struct {
			Code      int    `json:"code"`
			Message   string `json:"message"`
			ErrorCode int    `json:"error_code"`
		} `json:"status"`
	}
	if json.Unmarshal(body, &payload) == nil {
		apiErr.StatusCode = payload.Status.Code
		apiErr.ErrorCode = payload.Status.ErrorCode
		if apiErr.ErrorCode == 0 {
			apiErr.ErrorCode = payload.ErrorCode
		}
		apiErr.Message = payload.Status.Message
		if apiErr.Message == "" {
			apiErr.Message = payload.Message
		}
		if apiErr.Message == "" {
			apiErr.Message = payload.Error
		}
	}
	return apiErr
}

func IsInsufficientBalance(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.ErrorCode >= 40201 && apiErr.ErrorCode <= 40208 {
		return true
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "insufficient_balance") ||
		strings.Contains(text, "insufficient balance") ||
		strings.Contains(text, "insufficient credits")
}

func IsGenerationBaseURLMissing(err error) bool {
	return err != nil && strings.Contains(err.Error(), "generation base URL not configured")
}

func ErrNotLoggedIn() error {
	return &CLIError{
		Message: "not logged in",
		Hint:    "Run: seacloud auth login",
	}
}

func ErrTokenExpired() error {
	return &CLIError{
		Message: "session expired",
		Hint:    "Run: seacloud auth login",
	}
}

func ErrTokenInvalid() error {
	return &CLIError{
		Message: "invalid token",
		Hint:    "Run: seacloud auth login",
	}
}

func ErrTokenVerification(err error) error {
	return &CLIError{
		Message: fmt.Sprintf("token verification failed: %v", err),
		Hint:    "Run: seacloud auth login",
	}
}

func ErrSaveConfig(err error) error {
	return &CLIError{
		Message: fmt.Sprintf("failed to save config: %v", err),
		Hint:    "Check write permissions for ~/.config/seacloud/",
	}
}

func ErrLogout(err error) error {
	return &CLIError{
		Message: fmt.Sprintf("failed to clear credentials: %v", err),
		Hint:    "Try deleting ~/.config/seacloud/config.yml manually",
	}
}

func ErrNetwork(err error) error {
	return &CLIError{
		Message: fmt.Sprintf("network error: %v", err),
		Hint:    "Check your network connection and that the SeaCloud API is reachable",
	}
}

func ErrNetworkTimeout(err error) error {
	return &CLIError{
		Message: fmt.Sprintf("request timed out: %v", err),
		Hint:    "Check your network connection or try again",
	}
}

func ErrModelNotFound(id string) error {
	return &CLIError{
		Message: fmt.Sprintf("model %q not found", id),
		Hint:    "Run: seacloud models list to see available models",
	}
}

func ErrFetchModels(err error) error {
	return &CLIError{
		Message: fmt.Sprintf("failed to fetch models: %v", err),
		Hint:    "Check your network connection and try again",
	}
}

func ErrFetchModelSpec(id string, err error) error {
	return &CLIError{
		Message: fmt.Sprintf("failed to fetch spec for %q: %v", id, err),
		Hint:    "Run: seacloud models list to see available models",
	}
}

func ErrNoAPIKey() error {
	return &CLIError{
		Message: "API key not set",
		Hint:    "Run: seacloud auth login to obtain an API key, run seacloud auth set-key <api-key>, or inject FOLKOS_EXEC_TOKEN in managed runtimes",
	}
}

func ErrManagedCredentialsOverride() error {
	return &CLIError{
		Message: "credentials are managed by the runtime",
		Hint:    "Unset FOLKOS_EXEC_TOKEN to manage credentials locally with seacloud auth login or auth set-key",
	}
}

func ErrInvalidParam(modelID, name, reason string) error {
	return &CLIError{
		Message: fmt.Sprintf("invalid value for parameter %q: %s", name, reason),
		Hint:    fmt.Sprintf("Run: seacloud models spec %s to see allowed values", modelID),
	}
}

func ErrMissingParam(modelID, name string) error {
	return &CLIError{
		Message: fmt.Sprintf("missing required parameter: %q", name),
		Hint:    fmt.Sprintf("Run: seacloud models spec %s to see required parameters", modelID),
	}
}

func ErrSubmitFailed(err error) error {
	if IsInsufficientBalance(err) {
		return ErrInsufficientBalance()
	}
	if IsGenerationBaseURLMissing(err) {
		return &CLIError{
			Message: fmt.Sprintf("generation request failed: %v", err),
			Hint:    "Set SEACLOUD_GENERATION_URL=https://cloud.seaart.ai or install a release build with a configured generation endpoint.",
		}
	}
	return &CLIError{
		Message: fmt.Sprintf("generation request failed: %v", err),
		Hint:    "Check your API key with: seacloud auth status",
	}
}

func ErrInsufficientBalance() error {
	return &CLIError{
		Message: "insufficient balance",
		Hint:    "Check your balance: seacloud account balance\n        Top up at: https://cloud.seaart.ai/settings/credits",
	}
}

func ErrTaskFailed(taskID, reason string) error {
	return &CLIError{
		Message: fmt.Sprintf("task %s failed: %s", taskID, reason),
	}
}

func ErrTaskTimeout(taskID string) error {
	return &CLIError{
		Message: fmt.Sprintf("task %s timed out waiting for result", taskID),
		Hint:    fmt.Sprintf("Run: seacloud task status %s to check later", taskID),
	}
}
