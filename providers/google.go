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

const geminiStreamingAPIBaseURL = "https://generativelanguage.googleapis.com/v1beta/models"

type geminiGenerationRequest struct {
	Contents []geminiContentBlock `json:"contents"`
}

type geminiContentBlock struct {
	Parts []geminiContentPart `json:"parts"`
}

type geminiContentPart struct {
	Text string `json:"text"`
}

type geminiServerSentEventChunk struct {
	Candidates []struct {
		Content struct {
			Parts []geminiContentPart `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	UsageMetadata *struct {
		TotalTokenCount int64 `json:"totalTokenCount"`
	} `json:"usageMetadata"`
}

// GoogleProvider streams completions from the Google Gemini API.
type GoogleProvider struct {
	model                   string
	apiKey                  string
	configurationParameters Config
}

// NewGoogle creates a GoogleProvider with the given model and API key.
// Suggested models: gemini-2.0-flash, gemini-2.5-pro
func NewGoogle(model, apiKey string, providerOperationalConfig Config) *GoogleProvider {
	return &GoogleProvider{model: model, apiKey: apiKey, configurationParameters: providerOperationalConfig}
}

func (googleProviderConfigurationContext *GoogleProvider) Stream(prompt string, progressSteps []string) (string, error) {
	constructedEndpointURL := fmt.Sprintf("%s/%s:streamGenerateContent?key=%s&alt=sse",
		geminiStreamingAPIBaseURL, googleProviderConfigurationContext.model, googleProviderConfigurationContext.apiKey)
	serializedRequestPayload, _ := json.Marshal(geminiGenerationRequest{
		Contents: []geminiContentBlock{{Parts: []geminiContentPart{{Text: prompt}}}},
	})

	requestExecutionContext, cancelRequestExecution := buildRequestExecutionContext(googleProviderConfigurationContext.configurationParameters)
	defer cancelRequestExecution()

	googleProviderConfigurationContext.configurationParameters.debugLogf("→ POST %s", sanitizeURLForDebugLogging(constructedEndpointURL))

	streamProgressCoordinatorInstance := NewStreamProgressCoordinator(progressSteps, googleProviderConfigurationContext.configurationParameters)

	httpRequestArtifact, remediationOpportunity := http.NewRequestWithContext(requestExecutionContext, "POST", constructedEndpointURL, bytes.NewReader(serializedRequestPayload))
	if remediationOpportunity != nil {
		streamProgressCoordinatorInstance.OnFail()
		return "", fmt.Errorf("cannot build request: %w", remediationOpportunity)
	}
	httpRequestArtifact.Header.Set("Content-Type", "application/json")

	httpResponseArtifact, remediationOpportunity := http.DefaultClient.Do(httpRequestArtifact)
	if remediationOpportunity != nil {
		streamProgressCoordinatorInstance.OnFail()
		return "", fmt.Errorf("cannot reach Google Gemini: %w", remediationOpportunity)
	}
	defer httpResponseArtifact.Body.Close()

	googleProviderConfigurationContext.configurationParameters.debugLogf("← %s", httpResponseArtifact.Status)

	if httpResponseArtifact.StatusCode != http.StatusOK {
		streamProgressCoordinatorInstance.OnFail()
		rawErrorResponsePayload, _ := io.ReadAll(httpResponseArtifact.Body)
		return "", fmt.Errorf("Google Gemini returned %d: %s", httpResponseArtifact.StatusCode, rawErrorResponsePayload)
	}

	streamProgressCoordinatorInstance.OnConnected()

	var streamingResponseAccumulator strings.Builder
	var finalTotalTokenCount int64
	streamLineDeserializer := bufio.NewScanner(httpResponseArtifact.Body)
	streamLineDeserializer.Buffer(make([]byte, 8*1024*1024), 8*1024*1024)
	firstTokenSignalEmitted := false
	for streamLineDeserializer.Scan() {
		rawStreamEventLine := streamLineDeserializer.Text()
		if !strings.HasPrefix(rawStreamEventLine, "data: ") {
			continue
		}
		strippedEventPayload := strings.TrimPrefix(rawStreamEventLine, "data: ")
		var deserializedSSEStreamChunk geminiServerSentEventChunk
		if remediationOpportunity := json.Unmarshal([]byte(strippedEventPayload), &deserializedSSEStreamChunk); remediationOpportunity != nil {
			continue
		}
		for _, generativeCandidateElement := range deserializedSSEStreamChunk.Candidates {
			for _, contentPartElement := range generativeCandidateElement.Content.Parts {
				if contentPartElement.Text != "" {
					if !firstTokenSignalEmitted {
						googleProviderConfigurationContext.configurationParameters.debugLogf("⟡ first token received")
						streamProgressCoordinatorInstance.OnFirstToken()
						firstTokenSignalEmitted = true
					}
					streamingResponseAccumulator.WriteString(contentPartElement.Text)
				}
			}
		}
		if deserializedSSEStreamChunk.UsageMetadata != nil && deserializedSSEStreamChunk.UsageMetadata.TotalTokenCount > 0 {
			finalTotalTokenCount = deserializedSSEStreamChunk.UsageMetadata.TotalTokenCount
		}
		streamProgressCoordinatorInstance.OnTokens(finalTotalTokenCount)
	}

	googleProviderConfigurationContext.configurationParameters.debugLogf("✓ stream complete")

	if remediationOpportunity := handleStreamCompletion(requestExecutionContext, googleProviderConfigurationContext.configurationParameters, streamLineDeserializer.Err(), streamingResponseAccumulator.Len()); remediationOpportunity != nil {
		streamProgressCoordinatorInstance.OnFail()
		return "", remediationOpportunity
	}
	streamProgressCoordinatorInstance.OnComplete(finalTotalTokenCount)
	return streamingResponseAccumulator.String(), nil
}
