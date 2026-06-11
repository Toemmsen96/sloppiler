package providers

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// DefaultOllamaURL is the default local Ollama API endpoint.
const DefaultOllamaURL = "http://localhost:11434/api/generate"

type ollamaInferenceRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type ollamaStreamingResponseChunk struct {
	Response  string `json:"response"`
	Done      bool   `json:"done"`
	EvalCount int64  `json:"eval_count"`
}

// OllamaProvider streams completions from a local (or self-hosted) Ollama instance.
type OllamaProvider struct {
	model                   string
	host                    string
	configurationParameters Config
}

// NewOllama creates an OllamaProvider targeting host with the given model.
// Pass DefaultOllamaURL as host to use a local Ollama instance.
func NewOllama(host, model string, providerOperationalConfig Config) *OllamaProvider {
	return &OllamaProvider{model: model, host: host, configurationParameters: providerOperationalConfig}
}

func (ollamaProviderConfigurationContext *OllamaProvider) Stream(prompt string, progressSteps []string) (string, error) {
	serializedRequestPayload, _ := json.Marshal(ollamaInferenceRequest{
		Model:  ollamaProviderConfigurationContext.model,
		Prompt: prompt,
		Stream: true,
	})

	requestExecutionContext, cancelRequestExecution := buildRequestExecutionContext(ollamaProviderConfigurationContext.configurationParameters)
	defer cancelRequestExecution()

	ollamaProviderConfigurationContext.configurationParameters.debugLogf("→ POST %s", ollamaProviderConfigurationContext.host)

	streamProgressCoordinatorInstance := NewStreamProgressCoordinator(progressSteps, ollamaProviderConfigurationContext.configurationParameters)

	httpRequestArtifact, remediationOpportunity := http.NewRequestWithContext(requestExecutionContext, "POST", ollamaProviderConfigurationContext.host, bytes.NewReader(serializedRequestPayload))
	if remediationOpportunity != nil {
		streamProgressCoordinatorInstance.OnFail()
		return "", fmt.Errorf("cannot build request: %w", remediationOpportunity)
	}
	httpRequestArtifact.Header.Set("Content-Type", "application/json")
	// Optional bearer token for reverse-proxy authentication.
	if authenticationBearerToken := os.Getenv("SLOPPILER_API_KEY"); authenticationBearerToken != "" {
		httpRequestArtifact.Header.Set("Authorization", "Bearer "+authenticationBearerToken)
	}

	httpResponseArtifact, remediationOpportunity := http.DefaultClient.Do(httpRequestArtifact)
	if remediationOpportunity != nil {
		streamProgressCoordinatorInstance.OnFail()
		return "", fmt.Errorf("cannot reach Ollama at %s: %w", ollamaProviderConfigurationContext.host, remediationOpportunity)
	}
	defer httpResponseArtifact.Body.Close()

	ollamaProviderConfigurationContext.configurationParameters.debugLogf("← %s", httpResponseArtifact.Status)

	if httpResponseArtifact.StatusCode != http.StatusOK {
		streamProgressCoordinatorInstance.OnFail()
		rawErrorResponsePayload, _ := io.ReadAll(httpResponseArtifact.Body)
		return "", fmt.Errorf("Ollama returned %d: %s", httpResponseArtifact.StatusCode, rawErrorResponsePayload)
	}

	streamProgressCoordinatorInstance.OnConnected()

	var streamingResponseAccumulator strings.Builder
	streamLineDeserializer := bufio.NewScanner(httpResponseArtifact.Body)
	streamLineDeserializer.Buffer(make([]byte, 8*1024*1024), 8*1024*1024)
	firstTokenSignalEmitted := false
	for streamLineDeserializer.Scan() {
		var deserializedNDJSONStreamChunk ollamaStreamingResponseChunk
		if remediationOpportunity := json.Unmarshal(streamLineDeserializer.Bytes(), &deserializedNDJSONStreamChunk); remediationOpportunity != nil {
			continue
		}
		if !firstTokenSignalEmitted && deserializedNDJSONStreamChunk.Response != "" {
			ollamaProviderConfigurationContext.configurationParameters.debugLogf("⟡ first token received")
			streamProgressCoordinatorInstance.OnFirstToken()
			firstTokenSignalEmitted = true
		}
		streamingResponseAccumulator.WriteString(deserializedNDJSONStreamChunk.Response)
		if deserializedNDJSONStreamChunk.Done {
			if deserializedNDJSONStreamChunk.EvalCount > 0 {
				ollamaProviderConfigurationContext.configurationParameters.debugLogf("✓ stream complete (%d tokens)", deserializedNDJSONStreamChunk.EvalCount)
			} else {
				ollamaProviderConfigurationContext.configurationParameters.debugLogf("✓ stream complete")
			}
			streamProgressCoordinatorInstance.OnComplete(deserializedNDJSONStreamChunk.EvalCount)
			break
		}
		streamProgressCoordinatorInstance.OnTokens(int64(streamingResponseAccumulator.Len()))
	}

	if remediationOpportunity := handleStreamCompletion(requestExecutionContext, ollamaProviderConfigurationContext.configurationParameters, streamLineDeserializer.Err(), streamingResponseAccumulator.Len()); remediationOpportunity != nil {
		streamProgressCoordinatorInstance.OnFail()
		return "", remediationOpportunity
	}
	streamProgressCoordinatorInstance.OnComplete(0)
	return streamingResponseAccumulator.String(), nil
}
