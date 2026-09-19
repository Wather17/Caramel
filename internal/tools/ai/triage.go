package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// TriageResult representa a decisão do modelo de visão barato (gatekeeper) sobre
// se uma imagem deve ou não ser enviada para a etapa cara de coloração.
type TriageResult struct {
	ShouldColorize bool   `json:"should_colorize"`
	Reason         string `json:"reason"`
}

// TriageImage envia a imagem para um modelo de visão de baixo custo (padrão: Gemma 4 free tier)
// e retorna a decisão de triagem: colorir ou pular.
//
// A resposta esperada é um JSON estrito: {"should_colorize": bool, "reason": "..."}.
// Qualquer falha (rede, parse, status) retorna erro — o chamador decide o comportamento
// (no fluxo de coloração o padrão é fail-open: colorir mesmo assim).
func (c *Client) TriageImage(imagePath string, promptText string, modelOverride string) (*TriageResult, error) {
	return c.TriageImageContext(context.Background(), imagePath, promptText, modelOverride)
}

// TriageImageContext é a variante cancelável de TriageImage.
func (c *Client) TriageImageContext(ctx context.Context, imagePath string, promptText string, modelOverride string) (*TriageResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	model := DefaultTriageModel
	if modelOverride != "" {
		model = modelOverride
	}

	dataURL, err := encodeImageAsDataURL(imagePath)
	if err != nil {
		return nil, err
	}

	reqPayload := ChatCompletionRequest{
		Model: model,
		Messages: []ChatMessage{
			{
				Role: "user",
				Content: []ChatMessageContentPart{
					{
						Type: "text",
						Text: promptText,
					},
					{
						Type: "image_url",
						ImageURL: &ImageURLProperty{
							URL: dataURL,
						},
					},
				},
			},
		},
	}

	jsonData, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, fmt.Errorf("falha ao serializar requisição de triagem: %w", err)
	}
	correlationID := imageCorrelationID("triage", model, promptText, dataURL)
	attemptEvent := AttemptMetadata{
		Operation: "triage", Role: "triage", RequestedModel: model, EffectiveModel: model,
		Attempt: c.nextAttempt(correlationID), CorrelationID: correlationID, StartedAt: time.Now().UTC(), Status: "failure",
	}
	defer func() {
		if attemptEvent.Status != "success" && attemptEvent.ErrorClass == "" {
			attemptEvent.ErrorClass = "unknown"
		}
		attemptEvent.FinishedAt = time.Now().UTC()
		attemptEvent.DurationMS = attemptEvent.FinishedAt.Sub(attemptEvent.StartedAt).Milliseconds()
		c.emitAttemptMetadata(ctx, attemptEvent)
	}()

	req, err := http.NewRequestWithContext(ctx, "POST", OpenRouterAPIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("falha ao criar requisição HTTP de triagem: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("HTTP-Referer", "https://github.com/Wather17/Caramel")
	req.Header.Set("X-Title", "Caramel CLI")
	req.Header.Set("X-Caramel-Correlation-ID", correlationID)

	release, err := c.acquireRequest(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTPClient.Do(req)
	release()
	if err != nil {
		transportErr := &retryableError{err: fmt.Errorf("erro na comunicação com a API de triagem: %w", err)}
		attemptEvent.ErrorClass = ErrorClass(transportErr)
		return nil, transportErr
	}
	attemptEvent.StatusCode = resp.StatusCode
	attemptEvent.RequestID = firstRequestID(resp.Header)
	attemptEvent.RetryAfterMS = retryAfterMilliseconds(resp.Header.Get("Retry-After"))
	if resp.StatusCode == http.StatusRequestTimeout || resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError {
		attemptEvent.ErrorClass = "transient"
	} else if resp.StatusCode >= http.StatusBadRequest {
		attemptEvent.ErrorClass = "permanent"
	}
	c.emitRequestMetadataWithModel("triage", model, correlationID, resp)
	defer resp.Body.Close()

	bodyBytes, err := readLimitedBody(resp.Body, resp.ContentLength, DefaultMaxResponseBodyBytes, "resposta da triagem")
	if err != nil {
		return nil, fmt.Errorf("falha ao ler resposta da triagem: %w", err)
	}

	c.debugf("🔍 [DEBUG] Resposta Raw da Triagem (%d bytes; trecho):\n%s\n\n", len(bodyBytes), truncateForError(string(bodyBytes)))

	if resp.StatusCode != http.StatusOK {
		return nil, statusErrorWithHeaders(resp.StatusCode, resp.Header, bodyBytes)
	}

	var chatResp ChatCompletionResponse
	if err := json.Unmarshal(bodyBytes, &chatResp); err != nil {
		return nil, fmt.Errorf("falha ao decodificar JSON da resposta de triagem: %w", err)
	}
	attemptEvent.Usage = chatResp.Usage

	if chatResp.Error != nil {
		return nil, apiError(fmt.Sprintf("erro na API de triagem: %s", chatResp.Error.Message), chatResp.Error.Code)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("resposta vazia da API de triagem")
	}

	rawContent := fmt.Sprintf("%v", chatResp.Choices[0].Message.Content)
	result, parseErr := parseTriageResponse(rawContent, model)
	if parseErr == nil {
		attemptEvent.Status = "success"
	}
	return result, parseErr
}

// parseTriageResponse interpreta a resposta textual do modelo de triagem.
// Tolera respostas envoltas em blocos markdown (```json ... ```) e texto extra ao redor do JSON.
func parseTriageResponse(raw, model string) (*TriageResult, error) {
	structuredJSON, err := ParseStructuredObject(raw, "triagem", model)
	if err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(structuredJSON, &fields); err != nil {
		return nil, newContractOutputError("triagem", model, StructuredJSONObject, raw, "objeto com tipos incompatíveis")
	}
	shouldRaw, ok := fields["should_colorize"]
	if !ok {
		return nil, newContractOutputError("triagem", model, StructuredJSONObject, raw, "campo should_colorize ausente")
	}
	reasonRaw, hasReason := fields["reason"]
	var result TriageResult
	if err := json.Unmarshal(shouldRaw, &result.ShouldColorize); err != nil {
		return nil, newContractOutputError("triagem", model, StructuredJSONObject, raw, "campo should_colorize deve ser booleano")
	}
	if hasReason {
		if err := json.Unmarshal(reasonRaw, &result.Reason); err != nil {
			return nil, newContractOutputError("triagem", model, StructuredJSONObject, raw, "campo reason deve ser texto")
		}
	}
	if !result.ShouldColorize && strings.TrimSpace(result.Reason) == "" {
		return nil, newContractOutputError("triagem", model, StructuredJSONObject, raw, "reason é obrigatório quando should_colorize é false")
	}

	return &result, nil
}

// truncateForError limita o tamanho de trechos de resposta incluídos em mensagens de erro
func truncateForError(s string) string {
	const maxLen = 200
	s = strings.TrimSpace(s)
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}

// TriageSkipInfo descreve uma imagem pulada pela triagem (para relatórios no CLI/pipeline)
type TriageSkipInfo struct {
	Name   string
	Stage  string // "local" (saturação) ou "triage" (LLM)
	Reason string
}
