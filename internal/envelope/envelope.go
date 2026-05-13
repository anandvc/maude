package envelope

import (
	"encoding/json"
	"fmt"
	"strings"
)

type PrintRequest struct {
	RequestID   string `json:"request_id"`
	Message     string `json:"message"`
	RespondWith string `json:"respond_with"`
}

func BuildPrintRequest(req PrintRequest) (string, error) {
	payload := map[string]string{
		"kind":         "maude_print_request",
		"request_id":   req.RequestID,
		"message":      strings.TrimSpace(req.Message),
		"respond_with": req.RespondWith,
		"instruction":  fmt.Sprintf("Answer the message. For the final print-mode response only, pipe raw markdown/stdout to `%s`. Do not rely on visible tmux output as the response path.", req.RespondWith),
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
