package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type modelsPayload struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

func llmDiagnosticsSummary(config desktopRuntimeConfig) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	modelsURL := strings.TrimRight(config.LLMBaseURL, "/") + "/models"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsURL, nil)
	if err != nil {
		return fmt.Sprintf("Runtime: %s\nEndpoint: %s\nModel: %s\nStatus: invalid diagnostics request: %v", config.LLMRuntime, config.LLMBaseURL, config.LLMModel, err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return fmt.Sprintf("Runtime: %s\nEndpoint: %s\nModel: %s\nStatus: unavailable: %v", config.LLMRuntime, config.LLMBaseURL, config.LLMModel, err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Sprintf("Runtime: %s\nEndpoint: %s\nModel: %s\nStatus: %s", config.LLMRuntime, config.LLMBaseURL, config.LLMModel, response.Status)
	}

	var payload modelsPayload
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return fmt.Sprintf("Runtime: %s\nEndpoint: %s\nModel: %s\nStatus: connected, but model payload could not be decoded: %v", config.LLMRuntime, config.LLMBaseURL, config.LLMModel, err)
	}
	models := make([]string, 0, len(payload.Data))
	for _, model := range payload.Data {
		if strings.TrimSpace(model.ID) != "" {
			models = append(models, model.ID)
		}
	}
	if len(models) == 0 {
		return fmt.Sprintf("Runtime: %s\nEndpoint: %s\nModel: %s\nStatus: connected, no models reported", config.LLMRuntime, config.LLMBaseURL, config.LLMModel)
	}
	return fmt.Sprintf("Runtime: %s\nEndpoint: %s\nModel: %s\nStatus: connected\nAvailable models: %s", config.LLMRuntime, config.LLMBaseURL, config.LLMModel, strings.Join(models, ", "))
}
