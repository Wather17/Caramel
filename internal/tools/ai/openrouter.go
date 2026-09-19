package ai

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
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
	APIKey           string
	Verbose          bool
	DiagnosticWriter io.Writer
	HTTPClient       *http.Client
	URLPolicy        ImageURLPolicy
	MetadataWriter   func(RequestMetadata)
	AttemptWriter    func(AttemptMetadata)
	metadataMu       sync.Mutex
	attempts         map[string]int
	limiter          *requestLimiter
}

// RequestMetadata expõe apenas metadados redigidos da resposta do provedor.
type RequestMetadata struct {
	Operation     string
	Model         string
	CorrelationID string
	RequestID     string
	StatusCode    int
	RetryAfterMS  int64
	Usage         UsageMetadata
}

// NewClient cria uma nova instância do cliente OpenRouter
func NewClient(apiKey string) (*Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("chave de API do OpenRouter não configurada. Use 'caramel config setup' ou 'caramel config set openrouter_key <sua-chave>'")
	}
	return &Client{
		APIKey:           apiKey,
		DiagnosticWriter: os.Stderr,
		HTTPClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		limiter: newRequestLimiter(DefaultMaxConcurrentRequests),
	}, nil
}

// SetMaxConcurrentRequests ajusta o teto compartilhado de requests deste
// cliente. Valores não positivos restauram o padrão conservador.
func (c *Client) SetMaxConcurrentRequests(maxConcurrent int) {
	if c == nil {
		return
	}
	c.limiter = newRequestLimiter(maxConcurrent)
}

func (c *Client) acquireRequest(ctx context.Context) (func(), error) {
	if c == nil || c.limiter == nil {
		return func() {}, nil
	}
	if err := c.limiter.acquire(ctx); err != nil {
		return nil, err
	}
	return c.limiter.release, nil
}

// debugf envia diagnósticos somente para o canal configurado, nunca para stdout.
func (c *Client) debugf(format string, args ...interface{}) {
	if c == nil || !c.Verbose {
		return
	}
	writer := c.DiagnosticWriter
	if writer == nil {
		writer = io.Discard
	}
	_, _ = fmt.Fprintf(writer, format, args...)
}

func (c *Client) emitRequestMetadata(operation, correlationID string, resp *http.Response) {
	c.emitRequestMetadataWithModel(operation, "", correlationID, resp)
}

func (c *Client) emitRequestMetadataWithModel(operation, model, correlationID string, resp *http.Response) {
	if c == nil || resp == nil {
		return
	}
	requestID := firstRequestID(resp.Header)
	metadata := RequestMetadata{Operation: operation, Model: model, CorrelationID: correlationID, RequestID: requestID, StatusCode: resp.StatusCode, RetryAfterMS: retryAfterMilliseconds(resp.Header.Get("Retry-After"))}
	if c.MetadataWriter != nil {
		c.MetadataWriter(metadata)
	}
	if requestID != "" {
		c.debugf("[API] operação=%s status=%d request_id=%s correlação=%s\n", operation, resp.StatusCode, requestID, correlationID)
	}
}

func firstRequestID(headers http.Header) string {
	for _, key := range []string{"X-Request-ID", "X-Request-Id", "Request-Id"} {
		if value := strings.TrimSpace(headers.Get(key)); value != "" {
			return value
		}
	}
	return ""
}

func (c *Client) nextAttempt(correlationID string) int {
	if c == nil {
		return 1
	}
	c.metadataMu.Lock()
	defer c.metadataMu.Unlock()
	if c.attempts == nil {
		c.attempts = make(map[string]int)
	}
	c.attempts[correlationID]++
	return c.attempts[correlationID]
}

func (c *Client) emitAttemptMetadata(ctx context.Context, event AttemptMetadata) {
	writer := attemptWriterFromContext(ctx)
	if writer == nil && c != nil {
		writer = c.AttemptWriter
	}
	if writer == nil {
		return
	}
	writer(event.Redacted())
}

func retryAfterMilliseconds(header string) int64 {
	if delay, ok := parseRetryAfter(header, retryNow()); ok {
		return delay.Milliseconds()
	}
	return 0
}

func imageCorrelationID(parts ...string) string {
	hash := sha256.New()
	for _, part := range parts {
		_, _ = hash.Write([]byte(part))
		_, _ = hash.Write([]byte{0})
	}
	return "caramel-" + fmt.Sprintf("%x", hash.Sum(nil))[:48]
}

func requestTrace(ctx context.Context, wrote *bool) context.Context {
	return httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{WroteRequest: func(info httptrace.WroteRequestInfo) {
		if info.Err == nil {
			*wrote = true
		}
	}})
}

func ambiguousImageError(operation, correlationID string, err error) error {
	return &UnknownOutcomeError{Operation: operation, CorrelationID: correlationID, Err: err}
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
	Usage UsageMetadata `json:"usage,omitempty"`
}

// ColorizeImage envia uma imagem local usando um contexto de fundo.
func (c *Client) ColorizeImage(imagePath string, promptText string, modelOverride string) ([]byte, string, error) {
	return c.ColorizeImageContext(context.Background(), imagePath, promptText, modelOverride)
}

// ColorizeImageContext envia uma imagem local (PNG/JPEG/SVG) para a API do OpenRouter.
func (c *Client) ColorizeImageContext(ctx context.Context, imagePath string, promptText string, modelOverride string) ([]byte, string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	model := DefaultModel
	if modelOverride != "" {
		model = modelOverride
	}

	dataURL, err := encodeImageAsDataURL(imagePath)
	if err != nil {
		return nil, "", err
	}
	correlationID := imageCorrelationID("colorize", model, promptText, dataURL)
	attemptEvent := AttemptMetadata{
		Operation: "image_colorization", Role: "image", RequestedModel: model, EffectiveModel: model,
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

	req, err := http.NewRequestWithContext(ctx, "POST", OpenRouterAPIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, "", fmt.Errorf("falha ao criar requisição HTTP: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("HTTP-Referer", "https://github.com/Wather17/Caramel")
	req.Header.Set("X-Title", "Caramel CLI")
	req.Header.Set("Idempotency-Key", correlationID)
	req.Header.Set("X-Caramel-Correlation-ID", correlationID)
	wroteRequest := false
	req = req.WithContext(requestTrace(ctx, &wroteRequest))

	release, err := c.acquireRequest(ctx)
	if err != nil {
		return nil, "", err
	}
	resp, err := c.HTTPClient.Do(req)
	release()
	if err != nil {
		transportErr := &retryableError{err: fmt.Errorf("erro na comunicação com a API do OpenRouter: %w", err)}
		attemptEvent.ErrorClass = ErrorClass(transportErr)
		if wroteRequest {
			attemptEvent.ErrorClass = "unknown_outcome"
			return nil, "", ambiguousImageError("image_colorization", correlationID, transportErr)
		}
		return nil, "", transportErr
	}
	attemptEvent.StatusCode = resp.StatusCode
	attemptEvent.RequestID = firstRequestID(resp.Header)
	attemptEvent.RetryAfterMS = retryAfterMilliseconds(resp.Header.Get("Retry-After"))
	if resp.StatusCode == http.StatusRequestTimeout || resp.StatusCode >= http.StatusInternalServerError {
		attemptEvent.ErrorClass = "unknown_outcome"
	} else if resp.StatusCode == http.StatusTooManyRequests {
		attemptEvent.ErrorClass = "transient"
	} else if resp.StatusCode >= http.StatusBadRequest {
		attemptEvent.ErrorClass = "permanent"
	}
	c.emitRequestMetadataWithModel("image_colorization", model, correlationID, resp)
	defer resp.Body.Close()

	bodyBytes, err := readLimitedBody(resp.Body, resp.ContentLength, DefaultMaxResponseBodyBytes, "resposta da API")
	if err != nil {
		return nil, "", ambiguousImageError("image_colorization", correlationID, fmt.Errorf("falha ao ler resposta da API: %w", err))
	}

	c.debugf("🔍 [DEBUG] Resposta Raw do OpenRouter (%d bytes; trecho):\n%s\n\n", len(bodyBytes), truncateForError(string(bodyBytes)))

	if resp.StatusCode != http.StatusOK {
		statusErr := statusErrorWithHeaders(resp.StatusCode, resp.Header, bodyBytes)
		if resp.StatusCode == http.StatusRequestTimeout || resp.StatusCode >= http.StatusInternalServerError {
			return nil, "", ambiguousImageError("image_colorization", correlationID, statusErr)
		}
		return nil, "", statusErr
	}

	var chatResp ChatCompletionResponse
	if err := json.Unmarshal(bodyBytes, &chatResp); err != nil {
		return nil, "", ambiguousImageError("image_colorization", correlationID, fmt.Errorf("falha ao decodificar JSON da resposta: %w", err))
	}
	attemptEvent.Usage = chatResp.Usage

	if chatResp.Error != nil {
		apiErr := apiError(fmt.Sprintf("erro na API OpenRouter: %s", chatResp.Error.Message), chatResp.Error.Code)
		if chatResp.Error.Code != http.StatusTooManyRequests && isRetryable(apiErr) {
			return nil, "", ambiguousImageError("image_colorization", correlationID, apiErr)
		}
		return nil, "", apiErr
	}

	if len(chatResp.Choices) == 0 {
		return nil, "", ambiguousImageError("image_colorization", correlationID, fmt.Errorf("resposta vazia da API do OpenRouter"))
	}

	choice := chatResp.Choices[0]
	var extractionErr error

	// 1. Padrão oficial OpenRouter: `message.images[0].image_url.url`
	if len(choice.Message.Images) > 0 {
		for _, imgItem := range choice.Message.Images {
			if imgItem.ImageURL != nil && imgItem.ImageURL.URL != "" {
				bytes, ext, err := c.extractImageBytesFromResponseContext(ctx, imgItem.ImageURL.URL)
				if err == nil {
					attemptEvent.Status = "success"
					return bytes, ext, nil
				}
				extractionErr = err
			}
		}
	}

	// 2. Extrai se `choice.Message.Content` for um array de objetos multimodal
	if contentArray, ok := choice.Message.Content.([]interface{}); ok {
		for _, part := range contentArray {
			if partMap, ok := part.(map[string]interface{}); ok {
				if imgURLObj, ok := partMap["image_url"].(map[string]interface{}); ok {
					if urlStr, ok := imgURLObj["url"].(string); ok {
						bytes, ext, err := c.extractImageBytesFromResponseContext(ctx, urlStr)
						if err == nil {
							attemptEvent.Status = "success"
							return bytes, ext, nil
						}
						extractionErr = err
					}
				}
			}
		}
	}

	// 3. Fallback para string simples ou Markdown
	rawContent := fmt.Sprintf("%v", choice.Message.Content)
	outBytes, outExt, err := c.extractImageBytesFromResponseContext(ctx, rawContent)
	if err != nil {
		if extractionErr != nil {
			return nil, "", ambiguousImageError("image_colorization", correlationID, fmt.Errorf("falha ao extrair imagem da resposta da IA: %w", extractionErr))
		}
		return nil, "", ambiguousImageError("image_colorization", correlationID, fmt.Errorf("falha ao extrair imagem da resposta da IA: %w", err))
	}
	attemptEvent.Status = "success"

	return outBytes, outExt, nil
}

// GenerateImage envia um prompt de texto usando um contexto de fundo.
func (c *Client) GenerateImage(promptText string, modelOverride string, aspect string) ([]byte, string, error) {
	return c.GenerateImageContext(context.Background(), promptText, modelOverride, aspect)
}

// GenerateImageContext envia um prompt de texto diretamente para a API do OpenRouter.
// aspect define a proporção da imagem gerada via image_config (ex: "1:1", "16:9", "auto").
// Se vazio, assume "1:1" — o formato padrão do Caramel.
func (c *Client) GenerateImageContext(ctx context.Context, promptText string, modelOverride string, aspect string) ([]byte, string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	model := DefaultModel
	if modelOverride != "" {
		model = modelOverride
	}

	if aspect == "" {
		aspect = "1:1"
	}
	correlationID := imageCorrelationID("generate", model, aspect, promptText)
	attemptEvent := AttemptMetadata{
		Operation: "image_generation", Role: "image", RequestedModel: model, EffectiveModel: model,
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

	req, err := http.NewRequestWithContext(ctx, "POST", OpenRouterAPIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, "", fmt.Errorf("falha ao criar requisição HTTP: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("HTTP-Referer", "https://github.com/Wather17/Caramel")
	req.Header.Set("X-Title", "Caramel CLI")
	req.Header.Set("Idempotency-Key", correlationID)
	req.Header.Set("X-Caramel-Correlation-ID", correlationID)
	wroteRequest := false
	req = req.WithContext(requestTrace(ctx, &wroteRequest))

	release, err := c.acquireRequest(ctx)
	if err != nil {
		return nil, "", err
	}
	resp, err := c.HTTPClient.Do(req)
	release()
	if err != nil {
		transportErr := &retryableError{err: fmt.Errorf("erro na comunicação com a API do OpenRouter: %w", err)}
		attemptEvent.ErrorClass = ErrorClass(transportErr)
		if wroteRequest {
			attemptEvent.ErrorClass = "unknown_outcome"
			return nil, "", ambiguousImageError("image_generation", correlationID, transportErr)
		}
		return nil, "", transportErr
	}
	attemptEvent.StatusCode = resp.StatusCode
	attemptEvent.RequestID = firstRequestID(resp.Header)
	attemptEvent.RetryAfterMS = retryAfterMilliseconds(resp.Header.Get("Retry-After"))
	if resp.StatusCode == http.StatusRequestTimeout || resp.StatusCode >= http.StatusInternalServerError {
		attemptEvent.ErrorClass = "unknown_outcome"
	} else if resp.StatusCode == http.StatusTooManyRequests {
		attemptEvent.ErrorClass = "transient"
	} else if resp.StatusCode >= http.StatusBadRequest {
		attemptEvent.ErrorClass = "permanent"
	}
	c.emitRequestMetadataWithModel("image_generation", model, correlationID, resp)
	defer resp.Body.Close()

	bodyBytes, err := readLimitedBody(resp.Body, resp.ContentLength, DefaultMaxResponseBodyBytes, "resposta da API")
	if err != nil {
		return nil, "", ambiguousImageError("image_generation", correlationID, fmt.Errorf("falha ao ler resposta da API: %w", err))
	}

	c.debugf("🔍 [DEBUG] Resposta Raw do OpenRouter GenerateImage (%d bytes; trecho):\n%s\n\n", len(bodyBytes), truncateForError(string(bodyBytes)))

	if resp.StatusCode != http.StatusOK {
		statusErr := statusErrorWithHeaders(resp.StatusCode, resp.Header, bodyBytes)
		if resp.StatusCode == http.StatusRequestTimeout || resp.StatusCode >= http.StatusInternalServerError {
			return nil, "", ambiguousImageError("image_generation", correlationID, statusErr)
		}
		return nil, "", statusErr
	}

	var chatResp ChatCompletionResponse
	if err := json.Unmarshal(bodyBytes, &chatResp); err != nil {
		return nil, "", ambiguousImageError("image_generation", correlationID, fmt.Errorf("falha ao decodificar JSON da resposta: %w", err))
	}
	attemptEvent.Usage = chatResp.Usage

	if chatResp.Error != nil {
		apiErr := apiError(fmt.Sprintf("erro na API OpenRouter: %s", chatResp.Error.Message), chatResp.Error.Code)
		if chatResp.Error.Code != http.StatusTooManyRequests && isRetryable(apiErr) {
			return nil, "", ambiguousImageError("image_generation", correlationID, apiErr)
		}
		return nil, "", apiErr
	}

	if len(chatResp.Choices) == 0 {
		return nil, "", ambiguousImageError("image_generation", correlationID, fmt.Errorf("resposta vazia da API do OpenRouter"))
	}

	choice := chatResp.Choices[0]
	var extractionErr error

	// 1. Padrão oficial OpenRouter: `message.images[0].image_url.url`
	if len(choice.Message.Images) > 0 {
		for _, imgItem := range choice.Message.Images {
			if imgItem.ImageURL != nil && imgItem.ImageURL.URL != "" {
				bytes, ext, err := c.extractImageBytesFromResponseContext(ctx, imgItem.ImageURL.URL)
				if err == nil {
					attemptEvent.Status = "success"
					return bytes, ext, nil
				}
				extractionErr = err
			}
		}
	}

	// 2. Extrai se `choice.Message.Content` for um array de objetos multimodal
	if contentArray, ok := choice.Message.Content.([]interface{}); ok {
		for _, part := range contentArray {
			if partMap, ok := part.(map[string]interface{}); ok {
				if imgURLObj, ok := partMap["image_url"].(map[string]interface{}); ok {
					if urlStr, ok := imgURLObj["url"].(string); ok {
						bytes, ext, err := c.extractImageBytesFromResponseContext(ctx, urlStr)
						if err == nil {
							attemptEvent.Status = "success"
							return bytes, ext, nil
						}
						extractionErr = err
					}
				}
			}
		}
	}

	// 3. Fallback para string simples ou Markdown
	rawContent := fmt.Sprintf("%v", choice.Message.Content)
	outBytes, outExt, err := c.extractImageBytesFromResponseContext(ctx, rawContent)
	if err != nil {
		if extractionErr != nil {
			return nil, "", ambiguousImageError("image_generation", correlationID, fmt.Errorf("falha ao extrair imagem gerada da resposta da IA: %w", extractionErr))
		}
		return nil, "", ambiguousImageError("image_generation", correlationID, fmt.Errorf("falha ao extrair imagem gerada da resposta da IA: %w", err))
	}
	attemptEvent.Status = "success"

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

// extractImageBytesFromResponse extrai os bytes de imagem (Data URL, URL remota ou Base64 puro).
// A validação é feita por magic bytes (PNG/JPEG/WEBP), não por heurística de tamanho.
func (c *Client) extractImageBytesFromResponse(content string) ([]byte, string, error) {
	return c.extractImageBytesFromResponseContext(context.Background(), content)
}

func (c *Client) extractImageBytesFromResponseContext(ctx context.Context, content string) ([]byte, string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	var extractionErr error
	// 1. Procura por todas as ocorrências de Data URL (data:image/png;base64,...),
	//    tolerando case e aspas escapadas de JSON (\")
	for {
		idx := indexFold(content, "data:image/")
		if idx == -1 {
			break
		}

		dataPart := content[idx:]
		content = content[idx+1:] // avança a busca para a próxima ocorrência

		if endIdx := strings.IndexAny(dataPart, "\"' \n\r\t"); endIdx != -1 {
			dataPart = dataPart[:endIdx]
		}
		// Remove backslashes de escape de JSON (\") que ficaram presos no final
		dataPart = strings.TrimRight(dataPart, "\\")

		commaIdx := strings.Index(dataPart, ",")
		if commaIdx == -1 {
			extractionErr = fmt.Errorf("Data URL de imagem sem payload base64")
			continue
		}
		if !isSupportedDataImageHeader(dataPart[:commaIdx]) {
			extractionErr = fmt.Errorf("MIME da Data URL de imagem não é suportado; use image/png, image/jpeg ou image/webp")
			continue
		}

		b64Str := dataPart[commaIdx+1:]
		decBytes, err := base64.StdEncoding.DecodeString(b64Str)
		if err != nil {
			// Alguns modelos retornam base64 URL-safe sem padding
			decBytes, err = base64.RawURLEncoding.DecodeString(strings.TrimRight(b64Str, "="))
			if err != nil {
				extractionErr = fmt.Errorf("payload base64 da Data URL de imagem inválido")
				continue
			}
		}
		if ext, ok := detectImageType(decBytes); ok {
			return decBytes, ext, nil
		}
		extractionErr = fmt.Errorf("bytes da Data URL não correspondem a uma imagem PNG, JPEG ou WEBP")
	}

	// 2. Procura por URLs de imagens HTTP/HTTPS retornadas pela IA
	if match := urlRegex.FindString(content); match != "" {
		return c.downloadImageFromURLContext(ctx, match)
	}

	// 3. Fallback: Se for qualquer URL http/https simples na resposta
	if idx := strings.Index(content, "http://"); idx != -1 || strings.Index(content, "https://") != -1 {
		startIdx := strings.Index(content, "http")
		urlStr := content[startIdx:]
		if endIdx := strings.IndexAny(urlStr, " \"')\n"); endIdx != -1 {
			urlStr = urlStr[:endIdx]
		}
		return c.downloadImageFromURLContext(ctx, strings.TrimSpace(urlStr))
	}

	// 4. Fallback se for uma string base64 pura
	trimmed := strings.TrimSpace(content)
	decBytes, err := base64.StdEncoding.DecodeString(trimmed)
	if err == nil {
		if ext, ok := detectImageType(decBytes); ok {
			return decBytes, ext, nil
		}
	}

	if extractionErr != nil {
		return nil, "", extractionErr
	}
	return nil, "", fmt.Errorf("não foi possível extrair os dados da imagem")
}

func isSupportedDataImageHeader(header string) bool {
	header = strings.ToLower(strings.TrimSpace(header))
	return strings.HasPrefix(header, "data:image/png;base64") || strings.HasPrefix(header, "data:image/jpeg;base64") || strings.HasPrefix(header, "data:image/jpg;base64") || strings.HasPrefix(header, "data:image/webp;base64")
}

// indexFold retorna o índice da primeira ocorrência case-insensitive de needle em haystack
func indexFold(haystack, needle string) int {
	return strings.Index(strings.ToLower(haystack), strings.ToLower(needle))
}

// detectImageType identifica o formato da imagem pelos bytes mágicos (PNG/JPEG/WEBP)
func detectImageType(data []byte) (string, bool) {
	if len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}) {
		return "png", true
	}
	if len(data) >= 3 && bytes.Equal(data[:3], []byte{0xff, 0xd8, 0xff}) {
		return "jpg", true
	}
	if len(data) >= 12 && bytes.HasPrefix(data, []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")) {
		return "webp", true
	}
	return "", false
}

// downloadImageFromURL baixa os bytes de uma imagem via HTTP GET com limite de tamanho
// e validação por magic bytes
func (c *Client) downloadImageFromURL(url string) ([]byte, string, error) {
	return c.downloadImageFromURLContext(context.Background(), url)
}

func (c *Client) downloadImageFromURLContext(ctx context.Context, url string) ([]byte, string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	policy := c.imageURLPolicy()
	if err := validateImageURL(ctx, url, policy); err != nil {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	release, err := c.acquireRequest(ctx)
	if err != nil {
		return nil, "", err
	}
	resp, err := c.downloadHTTPClient().Do(req)
	release()
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", statusErrorWithHeaders(resp.StatusCode, resp.Header, nil)
	}

	data, err := readLimitedBody(resp.Body, resp.ContentLength, DefaultMaxDownloadBytes, "imagem baixada")
	if err != nil {
		return nil, "", err
	}

	ext, ok := detectImageType(data)
	if !ok {
		return nil, "", fmt.Errorf("conteúdo baixado não é uma imagem válida (PNG/JPEG/WEBP)")
	}

	return data, ext, nil
}

// AnalyzeRoutine sends plain text routine content using a context of background.
func (c *Client) AnalyzeRoutine(routineText string, promptText string, modelOverride string) (string, error) {
	return c.AnalyzeRoutineContext(context.Background(), routineText, promptText, modelOverride)
}

// AnalyzeRoutineContext sends plain text routine content to OpenRouter.
func (c *Client) AnalyzeRoutineContext(ctx context.Context, routineText string, promptText string, modelOverride string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
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
	correlationID := imageCorrelationID("analyze", model, routineText, promptText)
	attemptEvent := AttemptMetadata{
		Operation: "text_analysis", Role: "text", RequestedModel: model, EffectiveModel: model,
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
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("HTTP-Referer", "https://github.com/Wather17/Caramel")
	req.Header.Set("X-Title", "Caramel CLI")
	req.Header.Set("X-Caramel-Correlation-ID", correlationID)

	release, err := c.acquireRequest(ctx)
	if err != nil {
		return "", err
	}
	resp, err := c.HTTPClient.Do(req)
	release()
	if err != nil {
		transportErr := &retryableError{err: fmt.Errorf("failed to contact OpenRouter API: %w", err)}
		attemptEvent.ErrorClass = ErrorClass(transportErr)
		return "", transportErr
	}
	attemptEvent.StatusCode = resp.StatusCode
	attemptEvent.RequestID = firstRequestID(resp.Header)
	attemptEvent.RetryAfterMS = retryAfterMilliseconds(resp.Header.Get("Retry-After"))
	if resp.StatusCode == http.StatusRequestTimeout || resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError {
		attemptEvent.ErrorClass = "transient"
	} else if resp.StatusCode >= http.StatusBadRequest {
		attemptEvent.ErrorClass = "permanent"
	}
	c.emitRequestMetadataWithModel("text_analysis", model, correlationID, resp)
	defer resp.Body.Close()

	bodyBytes, err := readLimitedBody(resp.Body, resp.ContentLength, DefaultMaxResponseBodyBytes, "resposta da API")
	if err != nil {
		return "", fmt.Errorf("failed to read API response: %w", err)
	}

	c.debugf("🔍 [DEBUG] Resposta Raw de rotina do OpenRouter (%d bytes; trecho):\n%s\n\n", len(bodyBytes), truncateForError(string(bodyBytes)))

	if resp.StatusCode != http.StatusOK {
		return "", statusErrorWithHeaders(resp.StatusCode, resp.Header, bodyBytes)
	}

	var chatResp ChatCompletionResponse
	if err := json.Unmarshal(bodyBytes, &chatResp); err != nil {
		return "", fmt.Errorf("failed to decode JSON response: %w", err)
	}
	attemptEvent.Usage = chatResp.Usage

	if chatResp.Error != nil {
		return "", apiError(fmt.Sprintf("OpenRouter API error: %s", chatResp.Error.Message), chatResp.Error.Code)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("empty response from OpenRouter API")
	}
	attemptEvent.Status = "success"

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
