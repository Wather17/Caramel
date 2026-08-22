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
	"regexp"
	"strings"
	"time"
)

const (
	DefaultModel       = "google/gemini-3.1-flash-image-preview" // Google Nano Banana 2 (Gemini 3.1 Flash Image)
	DefaultTextModel   = "deepseek/deepseek-v4-flash"            // DeepSeek V4 Flash
	DefaultTriageModel = "qwen/qwen3.7-flash"                    // Qwen 3.7 Flash (visão, $0.03/M input)
)

// OpenRouterAPIURL é o endpoint de chat completions do OpenRouter.
// Declarado como var para permitir a substituição por servidor de teste (httptest).
var OpenRouterAPIURL = "https://openrouter.ai/api/v1/chat/completions"

var urlRegex = regexp.MustCompile(`https?://[^\s\)"']+\.(png|jpg|jpeg|webp)`)

// Client representa o cliente HTTP para a API do OpenRouter
type Client struct {
	APIKey     string
	Verbose    bool
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
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Modalities  []string      `json:"modalities,omitempty"`
	ImageConfig *ImageConfig  `json:"image_config,omitempty"`
}

// ImageConfig controla as dimensões da imagem gerada diretamente na requisição
// (parâmetro estruturado suportado pelo OpenRouter, mais confiável que instrução no prompt)
type ImageConfig struct {
	AspectRatio string `json:"aspect_ratio,omitempty"`
	ImageSize   string `json:"image_size,omitempty"`
}

type OpenRouterImageItem struct {
	Type     string            `json:"type"`
	ImageURL *ImageURLProperty `json:"image_url"`
}

type ChatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content interface{}           `json:"content"`
			Images  []OpenRouterImageItem `json:"images,omitempty"`
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

	dataURL, err := encodeImageAsDataURL(imagePath)
	if err != nil {
		return nil, "", err
	}

	reqPayload := ChatCompletionRequest{
		Model:      model,
		Modalities: []string{"image", "text"},
		Messages: []ChatMessage{
			{
				Role: "user",
				Content: []ChatMessageContentPart{
					{
						Type: "text",
						Text: promptText + "\n\nCRITICAL: Return the output image in your response payload. Do not return conversational text only.",
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

	if c.Verbose {
		fmt.Printf("🔍 [DEBUG] Resposta Raw do OpenRouter (%d bytes):\n%s\n\n", len(bodyBytes), string(bodyBytes))
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

	choice := chatResp.Choices[0]

	// 1. Padrão oficial OpenRouter: `message.images[0].image_url.url`
	if len(choice.Message.Images) > 0 {
		for _, imgItem := range choice.Message.Images {
			if imgItem.ImageURL != nil && imgItem.ImageURL.URL != "" {
				bytes, ext, err := c.extractImageBytesFromResponse(imgItem.ImageURL.URL)
				if err == nil {
					return bytes, ext, nil
				}
			}
		}
	}

	// 2. Extrai se `choice.Message.Content` for um array de objetos multimodal
	if contentArray, ok := choice.Message.Content.([]interface{}); ok {
		for _, part := range contentArray {
			if partMap, ok := part.(map[string]interface{}); ok {
				if imgURLObj, ok := partMap["image_url"].(map[string]interface{}); ok {
					if urlStr, ok := imgURLObj["url"].(string); ok {
						bytes, ext, err := c.extractImageBytesFromResponse(urlStr)
						if err == nil {
							return bytes, ext, nil
						}
					}
				}
			}
		}
	}

	// 3. Fallback para string simples ou Markdown
	rawContent := fmt.Sprintf("%v", choice.Message.Content)
	outBytes, outExt, err := c.extractImageBytesFromResponse(rawContent)
	if err != nil {
		return nil, "", fmt.Errorf("falha ao extrair imagem da resposta da IA: %w", err)
	}

	return outBytes, outExt, nil
}

// GenerateImage envia um prompt de texto diretamente para a API do OpenRouter e retorna os bytes da imagem gerada e sua extensão
// aspect define a proporção da imagem gerada via image_config (ex: "1:1", "16:9", "auto").
// Se vazio, assume "1:1" — o formato padrão do Caramel.
func (c *Client) GenerateImage(promptText string, modelOverride string, aspect string) ([]byte, string, error) {
	model := DefaultModel
	if modelOverride != "" {
		model = modelOverride
	}

	if aspect == "" {
		aspect = "1:1"
	}

	reqPayload := ChatCompletionRequest{
		Model:      model,
		Modalities: []string{"image", "text"},
		ImageConfig: &ImageConfig{
			AspectRatio: aspect,
		},
		Messages: []ChatMessage{
			{
				Role: "user",
				Content: []ChatMessageContentPart{
					{
						Type: "text",
						Text: promptText + "\n\nCRITICAL: Generate and return the visual image asset in your response payload.",
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

	if c.Verbose {
		fmt.Printf("🔍 [DEBUG] Resposta Raw do OpenRouter GenerateImage (%d bytes):\n%s\n\n", len(bodyBytes), string(bodyBytes))
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

	choice := chatResp.Choices[0]

	// 1. Padrão oficial OpenRouter: `message.images[0].image_url.url`
	if len(choice.Message.Images) > 0 {
		for _, imgItem := range choice.Message.Images {
			if imgItem.ImageURL != nil && imgItem.ImageURL.URL != "" {
				bytes, ext, err := c.extractImageBytesFromResponse(imgItem.ImageURL.URL)
				if err == nil {
					return bytes, ext, nil
				}
			}
		}
	}

	// 2. Extrai se `choice.Message.Content` for um array de objetos multimodal
	if contentArray, ok := choice.Message.Content.([]interface{}); ok {
		for _, part := range contentArray {
			if partMap, ok := part.(map[string]interface{}); ok {
				if imgURLObj, ok := partMap["image_url"].(map[string]interface{}); ok {
					if urlStr, ok := imgURLObj["url"].(string); ok {
						bytes, ext, err := c.extractImageBytesFromResponse(urlStr)
						if err == nil {
							return bytes, ext, nil
						}
					}
				}
			}
		}
	}

	// 3. Fallback para string simples ou Markdown
	rawContent := fmt.Sprintf("%v", choice.Message.Content)
	outBytes, outExt, err := c.extractImageBytesFromResponse(rawContent)
	if err != nil {
		return nil, "", fmt.Errorf("falha ao extrair imagem gerada da resposta da IA: %w", err)
	}

	return outBytes, outExt, nil
}

// encodeImageAsDataURL lê uma imagem local e a codifica como Data URL (base64) para envio multimodal
func encodeImageAsDataURL(imagePath string) (string, error) {
	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		return "", fmt.Errorf("falha ao ler arquivo de imagem '%s': %w", imagePath, err)
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
	return fmt.Sprintf("data:%s;base64,%s", mimeType, base64Image), nil
}

// extractImageBytesFromResponse extrai os bytes de imagem (Data URL, URL remota ou Base64 puro)
func (c *Client) extractImageBytesFromResponse(content string) ([]byte, string, error) {
	// 1. Procura por formato Data URL (data:image/png;base64,...)
	if idx := strings.Index(content, "data:image/"); idx != -1 {
		dataPart := content[idx:]
		if endIdx := strings.IndexAny(dataPart, `"' `); endIdx != -1 {
			dataPart = dataPart[:endIdx]
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
			if err == nil && len(decBytes) > 50 {
				return decBytes, ext, nil
			}
		}
	}

	// 2. Procura por URLs de imagens HTTP/HTTPS retornadas pela IA
	if match := urlRegex.FindString(content); match != "" {
		imgBytes, ext, err := c.downloadImageFromURL(match)
		if err == nil {
			return imgBytes, ext, nil
		}
	}

	// 3. Fallback: Se for qualquer URL http/https simples na resposta
	if idx := strings.Index(content, "http://"); idx != -1 || strings.Index(content, "https://") != -1 {
		startIdx := strings.Index(content, "http")
		urlStr := content[startIdx:]
		if endIdx := strings.IndexAny(urlStr, " \"')\n"); endIdx != -1 {
			urlStr = urlStr[:endIdx]
		}
		imgBytes, ext, err := c.downloadImageFromURL(strings.TrimSpace(urlStr))
		if err == nil {
			return imgBytes, ext, nil
		}
	}

	// 4. Fallback se for uma string base64 pura
	trimmed := strings.TrimSpace(content)
	decBytes, err := base64.StdEncoding.DecodeString(trimmed)
	if err == nil && len(decBytes) > 100 {
		return decBytes, "png", nil
	}

	return nil, "", fmt.Errorf("não foi possível extrair os dados da imagem")
}

// downloadImageFromURL baixa os bytes de uma imagem via HTTP GET
func (c *Client) downloadImageFromURL(url string) ([]byte, string, error) {
	resp, err := c.HTTPClient.Get(url)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("status HTTP %d ao baixar imagem da URL", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	ext := "png"
	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "jpeg") || strings.Contains(contentType, "jpg") {
		ext = "jpg"
	} else if strings.Contains(contentType, "webp") {
		ext = "webp"
	}

	return data, ext, nil
}

// AnalyzeRoutine sends plain text routine content to OpenRouter and returns the structured JSON response
func (c *Client) AnalyzeRoutine(routineText string, promptText string, modelOverride string) (string, error) {
	model := DefaultTextModel
	if modelOverride != "" {
		model = modelOverride
	}

	reqPayload := ChatCompletionRequest{
		Model: model,
		Messages: []ChatMessage{
			{
				Role: "user",
				Content: []ChatMessageContentPart{
					{
						Type: "text",
						Text: promptText + "\n\nRaw Routine Text:\n" + routineText,
					},
				},
			},
		},
	}

	jsonData, err := json.Marshal(reqPayload)
	if err != nil {
		return "", fmt.Errorf("failed to serialize request: %w", err)
	}

	req, err := http.NewRequest("POST", OpenRouterAPIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("HTTP-Referer", "https://github.com/Wather17/Caramel")
	req.Header.Set("X-Title", "Caramel CLI")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to contact OpenRouter API: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read API response: %w", err)
	}

	if c.Verbose {
		fmt.Printf("🔍 [DEBUG] Raw OpenRouter response (%d bytes):\n%s\n\n", len(bodyBytes), string(bodyBytes))
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("OpenRouter API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var chatResp ChatCompletionResponse
	if err := json.Unmarshal(bodyBytes, &chatResp); err != nil {
		return "", fmt.Errorf("failed to decode JSON response: %w", err)
	}

	if chatResp.Error != nil {
		return "", fmt.Errorf("OpenRouter API error: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("empty response from OpenRouter API")
	}

	choice := chatResp.Choices[0]
	rawContent := fmt.Sprintf("%v", choice.Message.Content)

	// Clean codeblock markers if output is wrapped in ```json ... ```
	cleaned := strings.TrimSpace(rawContent)
	if strings.HasPrefix(cleaned, "```") {
		lines := strings.Split(cleaned, "\n")
		var contentLines []string
		for _, line := range lines {
			trimmedLine := strings.TrimSpace(line)
			if !strings.HasPrefix(trimmedLine, "```") {
				contentLines = append(contentLines, line)
			}
		}
		cleaned = strings.Join(contentLines, "\n")
	}

	return strings.TrimSpace(cleaned), nil
}

