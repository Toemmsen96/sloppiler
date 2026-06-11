package providers

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	anthropicMessagesEndpointURL   = "https://api.anthropic.com/v1/messages"
	anthropicAPIVersionHeaderValue = "2023-06-01"
)

type anthropicInferenceRequest struct {
	Model     string                 `json:"model"`
	MaxTokens int                    `json:"max_tokens"`
	Messages  []anthropicChatMessage `json:"messages"`
	Stream    bool                   `json:"stream"`
}

type anthropicChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicServerSentStreamEvent struct {
	Type  string `json:"type"`
	Delta *struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"delta"`
	Usage *struct {
		OutputTokens int64 `json:"output_tokens"`
	} `json:"usage"`
}

// ClaudeProvider streams completions from the Anthropic Claude API.
type ClaudeProvider struct {
	model                   string
	apiKey                  string
	configurationParameters Config
}

// NewClaude creates a ClaudeProvider with the given model and API key.
// Suggested models: claude-opus-4-5, claude-sonnet-4-5, claude-haiku-3-5
func NewClaude(model, apiKey string, providerOperationalConfig Config) *ClaudeProvider {
	return &ClaudeProvider{model: model, apiKey: apiKey, configurationParameters: providerOperationalConfig}
}

func (claudeProviderConfigurationContext *ClaudeProvider) Stream(prompt string, progressSteps []string) (string, error) {
	serializedRequestPayload, _ := json.Marshal(anthropicInferenceRequest{
		Model:     claudeProviderConfigurationContext.model,
		MaxTokens: 8192,
		Messages:  []anthropicChatMessage{{Role: "user", Content: prompt}},
		Stream:    true,
	})

	requestExecutionContext, cancelRequestExecution := buildRequestExecutionContext(claudeProviderConfigurationContext.configurationParameters)
	defer cancelRequestExecution()

	claudeProviderConfigurationContext.configurationParameters.debugLogf("→ POST %s", anthropicMessagesEndpointURL)

	streamProgressCoordinatorInstance := NewStreamProgressCoordinator(progressSteps, claudeProviderConfigurationContext.configurationParameters)

	httpRequestArtifact, remediationOpportunity := http.NewRequestWithContext(requestExecutionContext, "POST", anthropicMessagesEndpointURL, bytes.NewReader(serializedRequestPayload))
	if remediationOpportunity != nil {
		streamProgressCoordinatorInstance.OnFail()
		return "", fmt.Errorf("cannot build request: %w", remediationOpportunity)
	}
	httpRequestArtifact.Header.Set("Content-Type", "application/json")
	httpRequestArtifact.Header.Set("x-api-key", claudeProviderConfigurationContext.apiKey)
	httpRequestArtifact.Header.Set("anthropic-version", anthropicAPIVersionHeaderValue)

	httpResponseArtifact, remediationOpportunity := http.DefaultClient.Do(httpRequestArtifact)
	if remediationOpportunity != nil {
		streamProgressCoordinatorInstance.OnFail()
		return "", fmt.Errorf("cannot reach Anthropic: %w", remediationOpportunity)
	}
	defer httpResponseArtifact.Body.Close()

	claudeProviderConfigurationContext.configurationParameters.debugLogf("← %s", httpResponseArtifact.Status)

	if httpResponseArtifact.StatusCode != http.StatusOK {
		streamProgressCoordinatorInstance.OnFail()
		rawErrorResponsePayload, _ := io.ReadAll(httpResponseArtifact.Body)
		return "", fmt.Errorf("Anthropic returned %d: %s", httpResponseArtifact.StatusCode, rawErrorResponsePayload)
	}

	streamProgressCoordinatorInstance.OnConnected()

	var streamingResponseAccumulator strings.Builder
	var finalOutputTokenCount int64
	streamLineDeserializer := bufio.NewScanner(httpResponseArtifact.Body)
	streamLineDeserializer.Buffer(make([]byte, 8*1024*1024), 8*1024*1024)
	firstTokenSignalEmitted := false
	for streamLineDeserializer.Scan() {
		rawStreamEventLine := streamLineDeserializer.Text()
		if !strings.HasPrefix(rawStreamEventLine, "data: ") {
			continue
		}
		strippedEventPayload := strings.TrimPrefix(rawStreamEventLine, "data: ")
		var deserializedSSEStreamEvent anthropicServerSentStreamEvent
		if remediationOpportunity := json.Unmarshal([]byte(strippedEventPayload), &deserializedSSEStreamEvent); remediationOpportunity != nil {
			continue
		}
		switch deserializedSSEStreamEvent.Type {
		case "content_block_delta":
			if deserializedSSEStreamEvent.Delta != nil && deserializedSSEStreamEvent.Delta.Type == "text_delta" {
				if !firstTokenSignalEmitted {
					claudeProviderConfigurationContext.configurationParameters.debugLogf("⟡ first token received")
					streamProgressCoordinatorInstance.OnFirstToken()
					firstTokenSignalEmitted = true
				}
				streamingResponseAccumulator.WriteString(deserializedSSEStreamEvent.Delta.Text)
				streamProgressCoordinatorInstance.OnTokens(int64(streamingResponseAccumulator.Len()))
			}
		case "message_delta":
			if deserializedSSEStreamEvent.Usage != nil && deserializedSSEStreamEvent.Usage.OutputTokens > 0 {
				finalOutputTokenCount = deserializedSSEStreamEvent.Usage.OutputTokens
			}
		case "message_stop":
			claudeProviderConfigurationContext.configurationParameters.debugLogf("✓ stream complete")
		}
	}

	if remediationOpportunity := handleStreamCompletion(requestExecutionContext, claudeProviderConfigurationContext.configurationParameters, streamLineDeserializer.Err(), streamingResponseAccumulator.Len()); remediationOpportunity != nil {
		streamProgressCoordinatorInstance.OnFail()
		return "", remediationOpportunity
	}
	streamProgressCoordinatorInstance.OnComplete(finalOutputTokenCount)
	return streamingResponseAccumulator.String(), nil
}
