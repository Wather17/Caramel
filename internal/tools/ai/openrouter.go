package ai

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	OpenRouterAPIURL = "https://openrouter.ai/api/v1/chat/completions"
	DefaultModel     = "google/nano-banana-2"
)

// Client representa o cliente HTTP para a API do OpenRouter
type Client struct {
	APIKey     string
	HTTPClient *http.Client
}

// NewClient cria uma nova instância do cliente OpenRouter
func NewClient(apiKey string) (*Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("chave de API do OpenRouter não configurada. Use 'caramel config setup' ou 'caramel config set openrouter_key <sua-chave>'")
	}
	return &Client{
		APIKey: apiKey,
		HTTPClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}, nil
}

type ChatMessageContentPart struct {
	Type     string            `json:"type"`
	Text     string            `json:"text,omitempty"`
	ImageURL *ImageURLProperty `json:"image_url,omitempty"`
}

type ImageURLProperty struct {
	URL string `json:"url"`
}

type ChatMessage struct {
	Role    string                   `json:"role"`
	Content []ChatMessageContentPart `json:"content"`
}

type ChatCompletionRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
}

type ChatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content interface{} `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"error,omitempty"`
}

// ColorizeImage envia uma imagem local (PNG/JPEG/SVG) para a API do OpenRouter e retorna a imagem colorida em bytes
func (c *Client) ColorizeImage(imagePath string, promptText string, modelOverride string) ([]byte, string, error) {
	model := DefaultModel
	if modelOverride != "" {
		model = modelOverride
	}

	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		return nil, "", fmt.Errorf("falha ao ler arquivo de imagem '%s': %w", imagePath, err)
	}

	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(imagePath), "."))
	mimeType := "image/png"
	switch ext {
	case "jpg", "jpeg":
		mimeType = "image/jpeg"
	case "webp":
		mimeType = "image/webp"
	case "svg":
		mimeType = "image/svg+xml"
	}

	base64Image := base64.StdEncoding.EncodeToString(imageData)
	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Image)

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
		return nil, "", fmt.Errorf("falha ao serializar requisição JSON: %w", err)
	}

	req, err := http.NewRequest("POST", OpenRouterAPIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, "", fmt.Errorf("falha ao criar requisição HTTP: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("HTTP-Referer", "https://github.com/Wather17/Caramel")
	req.Header.Set("X-Title", "Caramel CLI")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("erro na comunicação com a API do OpenRouter: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("falha ao ler resposta da API: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("API OpenRouter retornou status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var chatResp ChatCompletionResponse
	if err := json.Unmarshal(bodyBytes, &chatResp); err != nil {
		return nil, "", fmt.Errorf("falha ao decodificar JSON da resposta: %w", err)
	}

	if chatResp.Error != nil {
		return nil, "", fmt.Errorf("erro na API OpenRouter: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return nil, "", fmt.Errorf("resposta vazia da API do OpenRouter")
	}

	// Trata o conteúdo da resposta (pode ser string com base64 ou URL)
	rawContent := fmt.Sprintf("%v", chatResp.Choices[0].Message.Content)
	outBytes, outExt, err := extractImageBytesFromResponse(rawContent)
	if err != nil {
		return nil, "", err
	}

	return outBytes, outExt, nil
}

// extractImageBytesFromResponse extrai os bytes de imagem a partir do texto de resposta da IA (base64 ou data URL)
func extractImageBytesFromResponse(content string) ([]byte, string, error) {
	// Procura por formato data:image/png;base64,...
	if idx := strings.Index(content, "data:image/"); idx != -1 {
		dataPart := content[idx:]
		if endIdx := strings.Index(dataPart, `"`); endIdx != -1 {
			dataPart = dataPart[:endIdx]
		}
		if spaceIdx := strings.Index(dataPart, " "); spaceIdx != -1 {
			dataPart = dataPart[:spaceIdx]
		}

		commaIdx := strings.Index(dataPart, ",")
		if commaIdx != -1 {
			header := dataPart[:commaIdx]
			b64Str := dataPart[commaIdx+1:]
			ext := "png"
			if strings.Contains(header, "jpeg") || strings.Contains(header, "jpg") {
				ext = "jpg"
			}
			decBytes, err := base64.StdEncoding.DecodeString(b64Str)
			if err == nil {
				return decBytes, ext, nil
			}
		}
	}

	// Fallback se a resposta for pura string base64 sem cabeçalho data:
	decBytes, err := base64.StdEncoding.DecodeString(strings.TrimSpace(content))
	if err == nil && len(decBytes) > 100 {
		return decBytes, "png", nil
	}

	return nil, "", fmt.Errorf("não foi possível extrair os dados da imagem colorida da resposta da IA")
}
