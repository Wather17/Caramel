package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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

	req, err := http.NewRequest("POST", OpenRouterAPIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("falha ao criar requisição HTTP de triagem: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("HTTP-Referer", "https://github.com/Wather17/Caramel")
	req.Header.Set("X-Title", "Caramel CLI")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, &retryableError{err: fmt.Errorf("erro na comunicação com a API de triagem: %w", err)}
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("falha ao ler resposta da triagem: %w", err)
	}

	if c.Verbose {
		fmt.Printf("🔍 [DEBUG] Resposta Raw da Triagem (%d bytes):\n%s\n\n", len(bodyBytes), string(bodyBytes))
	}

	if resp.StatusCode != http.StatusOK {
		return nil, statusError(resp.StatusCode, bodyBytes)
	}

	var chatResp ChatCompletionResponse
	if err := json.Unmarshal(bodyBytes, &chatResp); err != nil {
		return nil, fmt.Errorf("falha ao decodificar JSON da resposta de triagem: %w", err)
	}

	if chatResp.Error != nil {
		return nil, fmt.Errorf("erro na API de triagem: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("resposta vazia da API de triagem")
	}

	rawContent := fmt.Sprintf("%v", chatResp.Choices[0].Message.Content)
	return parseTriageResponse(rawContent)
}

// parseTriageResponse interpreta a resposta textual do modelo de triagem.
// Tolera respostas envoltas em blocos markdown (```json ... ```) e texto extra ao redor do JSON.
func parseTriageResponse(raw string) (*TriageResult, error) {
	cleaned := strings.TrimSpace(raw)

	// Remove cercas de bloco de código markdown, se presentes
	if strings.HasPrefix(cleaned, "```") {
		lines := strings.Split(cleaned, "\n")
		var contentLines []string
		for _, line := range lines {
			if !strings.HasPrefix(strings.TrimSpace(line), "```") {
				contentLines = append(contentLines, line)
			}
		}
		cleaned = strings.TrimSpace(strings.Join(contentLines, "\n"))
	}

	// Extrai apenas o objeto JSON (primeiro '{' até o último '}')
	startIdx := strings.Index(cleaned, "{")
	endIdx := strings.LastIndex(cleaned, "}")
	if startIdx == -1 || endIdx == -1 || endIdx <= startIdx {
		return nil, fmt.Errorf("nenhum objeto JSON encontrado na resposta de triagem: %q", truncateForError(raw))
	}

	var result TriageResult
	if err := json.Unmarshal([]byte(cleaned[startIdx:endIdx+1]), &result); err != nil {
		return nil, fmt.Errorf("falha ao interpretar JSON da triagem: %w (conteúdo: %q)", err, truncateForError(raw))
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
