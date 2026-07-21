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
	OpenRouterAPIURL = "https://openrouter.ai/api/v1/chat/completions"
	DefaultModel     = "google/gemini-2.5-flash-image" // Google Nano Banana
)

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
	Model      string        `json:"model"`
	Messages   []ChatMessage `json:"messages"`
	Modalities []string      `json:"modalities,omitempty"`
}

type ChatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content interface{}   `json:"content"`
			Images  []interface{} `json:"images,omitempty"`
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
		fmt.Printf("🔍 [DEBUG] Resposta Raw do OpenRouter:\n%s\n\n", string(bodyBytes))
	}

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("API OpenRouter retornou status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var chatResp ChatCompletionResponse
	if err := json.Unmarshal(bodyBytes, &chatResp); err != nil {
		return nil, "", fmt.Errorf("falha ao decodificar JSON da resposta (Raw JSON: %s): %w", string(bodyBytes), err)
	}

	if chatResp.Error != nil {
		return nil, "", fmt.Errorf("erro na API OpenRouter: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return nil, "", fmt.Errorf("resposta vazia da API do OpenRouter. Raw: %s", string(bodyBytes))
	}

	choice := chatResp.Choices[0]

	// 1. Checa se o OpenRouter retornou no campo `message.images`
	if len(choice.Message.Images) > 0 {
		for _, imgObj := range choice.Message.Images {
			imgStr := fmt.Sprintf("%v", imgObj)
			bytes, ext, err := c.extractImageBytesFromResponse(imgStr)
			if err == nil {
				return bytes, ext, nil
			}
		}
	}

	// 2. Checa o conteúdo raw
	rawContent := fmt.Sprintf("%v", choice.Message.Content)
	outBytes, outExt, err := c.extractImageBytesFromResponse(rawContent)
	if err != nil {
		return nil, "", fmt.Errorf("falha ao extrair imagem da resposta (Raw JSON: %s): %w", string(bodyBytes), err)
	}

	return outBytes, outExt, nil
}

// extractImageBytesFromResponse extrai os bytes de imagem (Data URL, URL remota ou Base64 puro)
func (c *Client) extractImageBytesFromResponse(content string) ([]byte, string, error) {
	// 1. Procura por formato Data URL (data:image/png;base64,...)
	if idx := strings.Index(content, "data:image/"); idx != -1 {
		dataPart := content[idx:]
		if endIdx := strings.IndexAny(dataPart, `"'\ `); endIdx != -1 {
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
