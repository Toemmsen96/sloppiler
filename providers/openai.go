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

const openAIChatCompletionsEndpointURL = "https://api.openai.com/v1/chat/completions"

type openAIInferenceRequest struct {
	Model         string                        `json:"model"`
	Messages      []openAIChatHistoryMessage    `json:"messages"`
	Stream        bool                          `json:"stream"`
	StreamOptions *openAIStreamingOutputOptions `json:"stream_options,omitempty"`
}

type openAIStreamingOutputOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type openAIChatHistoryMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIServerSentEventChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
	Usage *struct {
		CompletionTokens int64 `json:"completion_tokens"`
	} `json:"usage"`
}

// OpenAIProvider streams completions from the OpenAI Chat Completions API.
type OpenAIProvider struct {
	model                   string
	apiKey                  string
	configurationParameters Config
}

// NewOpenAI creates an OpenAIProvider with the given model and API key.
// Suggested models: gpt-4o, gpt-4o-mini, o4-mini
func NewOpenAI(model, apiKey string, providerOperationalConfig Config) *OpenAIProvider {
	return &OpenAIProvider{model: model, apiKey: apiKey, configurationParameters: providerOperationalConfig}
}

func (openAIProviderConfigurationContext *OpenAIProvider) Stream(prompt string, progressSteps []string) (string, error) {
	serializedRequestPayload, _ := json.Marshal(openAIInferenceRequest{
		Model:         openAIProviderConfigurationContext.model,
		Messages:      []openAIChatHistoryMessage{{Role: "user", Content: prompt}},
		Stream:        true,
		StreamOptions: &openAIStreamingOutputOptions{IncludeUsage: true},
	})

	requestExecutionContext, cancelRequestExecution := buildRequestExecutionContext(openAIProviderConfigurationContext.configurationParameters)
	defer cancelRequestExecution()

	openAIProviderConfigurationContext.configurationParameters.debugLogf("→ POST %s", openAIChatCompletionsEndpointURL)

	streamProgressCoordinatorInstance := NewStreamProgressCoordinator(progressSteps, openAIProviderConfigurationContext.configurationParameters)

	httpRequestArtifact, remediationOpportunity := http.NewRequestWithContext(requestExecutionContext, "POST", openAIChatCompletionsEndpointURL, bytes.NewReader(serializedRequestPayload))
	if remediationOpportunity != nil {
		streamProgressCoordinatorInstance.OnFail()
		return "", fmt.Errorf("cannot build request: %w", remediationOpportunity)
	}
	httpRequestArtifact.Header.Set("Content-Type", "application/json")
	httpRequestArtifact.Header.Set("Authorization", "Bearer "+openAIProviderConfigurationContext.apiKey)

	httpResponseArtifact, remediationOpportunity := http.DefaultClient.Do(httpRequestArtifact)
	if remediationOpportunity != nil {
		streamProgressCoordinatorInstance.OnFail()
		return "", fmt.Errorf("cannot reach OpenAI: %w", remediationOpportunity)
	}
	defer httpResponseArtifact.Body.Close()

	openAIProviderConfigurationContext.configurationParameters.debugLogf("← %s", httpResponseArtifact.Status)

	if httpResponseArtifact.StatusCode != http.StatusOK {
		streamProgressCoordinatorInstance.OnFail()
		rawErrorResponsePayload, _ := io.ReadAll(httpResponseArtifact.Body)
		return "", fmt.Errorf("OpenAI returned %d: %s", httpResponseArtifact.StatusCode, rawErrorResponsePayload)
	}

	streamProgressCoordinatorInstance.OnConnected()

	var streamingResponseAccumulator strings.Builder
	streamLineDeserializer := bufio.NewScanner(httpResponseArtifact.Body)
	streamLineDeserializer.Buffer(make([]byte, 8*1024*1024), 8*1024*1024)
	firstTokenSignalEmitted := false
	for streamLineDeserializer.Scan() {
		rawStreamEventLine := streamLineDeserializer.Text()
		if !strings.HasPrefix(rawStreamEventLine, "data: ") {
			continue
		}
		strippedEventPayload := strings.TrimPrefix(rawStreamEventLine, "data: ")
		if strippedEventPayload == "[DONE]" {
			openAIProviderConfigurationContext.configurationParameters.debugLogf("✓ stream complete")
			break
		}
		var deserializedSSEStreamChunk openAIServerSentEventChunk
		if remediationOpportunity := json.Unmarshal([]byte(strippedEventPayload), &deserializedSSEStreamChunk); remediationOpportunity != nil {
			continue
		}
		for _, candidateChoiceElement := range deserializedSSEStreamChunk.Choices {
			if candidateChoiceElement.Delta.Content != "" {
				if !firstTokenSignalEmitted {
					openAIProviderConfigurationContext.configurationParameters.debugLogf("⟡ first token received")
					streamProgressCoordinatorInstance.OnFirstToken()
					firstTokenSignalEmitted = true
				}
				streamingResponseAccumulator.WriteString(candidateChoiceElement.Delta.Content)
			}
		}
		if deserializedSSEStreamChunk.Usage != nil && deserializedSSEStreamChunk.Usage.CompletionTokens > 0 {
			streamProgressCoordinatorInstance.OnComplete(deserializedSSEStreamChunk.Usage.CompletionTokens)
		} else {
			streamProgressCoordinatorInstance.OnTokens(int64(streamingResponseAccumulator.Len()))
		}
	}

	if remediationOpportunity := handleStreamCompletion(requestExecutionContext, openAIProviderConfigurationContext.configurationParameters, streamLineDeserializer.Err(), streamingResponseAccumulator.Len()); remediationOpportunity != nil {
		streamProgressCoordinatorInstance.OnFail()
		return "", remediationOpportunity
	}
	streamProgressCoordinatorInstance.OnComplete(0)
	return streamingResponseAccumulator.String(), nil
}
