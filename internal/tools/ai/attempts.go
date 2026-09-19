package ai

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"
)

// AttemptSchemaVersion é a versão do contrato local de metadados de tentativas.
const AttemptSchemaVersion = 1

// UsageMetadata contém somente os números de uso que o provedor expôs.
// Campos ausentes permanecem zero e são omitidos na serialização.
type UsageMetadata struct {
	PromptTokens     int     `json:"prompt_tokens,omitempty"`
	CompletionTokens int     `json:"completion_tokens,omitempty"`
	TotalTokens      int     `json:"total_tokens,omitempty"`
	ImageTokens      int     `json:"image_tokens,omitempty"`
	Cost             float64 `json:"cost,omitempty"`
}

// AttemptMetadata é o evento redigido persistido por execução e item.
// Não contém prompt, corpo HTTP, resposta bruta, credencial ou chave de API.
type AttemptMetadata struct {
	SchemaVersion   int           `json:"schema_version"`
	Sequence        int64         `json:"sequence,omitempty"`
	Operation       string        `json:"operation"`
	Role            string        `json:"role,omitempty"`
	RequestedModel  string        `json:"requested_model,omitempty"`
	EffectiveModel  string        `json:"effective_model,omitempty"`
	FallbackOrdinal int           `json:"fallback_ordinal,omitempty"`
	Attempt         int           `json:"attempt,omitempty"`
	ItemIndex       int           `json:"item_index,omitempty"`
	ItemName        string        `json:"item_name,omitempty"`
	ArtifactPath    string        `json:"artifact_path,omitempty"`
	Reused          bool          `json:"reused,omitempty"`
	StartedAt       time.Time     `json:"started_at,omitempty"`
	FinishedAt      time.Time     `json:"finished_at,omitempty"`
	DurationMS      int64         `json:"duration_ms,omitempty"`
	Status          string        `json:"status"`
	ErrorClass      string        `json:"error_class,omitempty"`
	StatusCode      int           `json:"status_code,omitempty"`
	RetryAfterMS    int64         `json:"retry_after_ms,omitempty"`
	RequestID       string        `json:"request_id,omitempty"`
	CorrelationID   string        `json:"correlation_id,omitempty"`
	Usage           UsageMetadata `json:"usage,omitempty"`
}

// Redacted devolve uma cópia segura para persistência e saída JSON.
func (m AttemptMetadata) Redacted() AttemptMetadata {
	if m.SchemaVersion == 0 {
		m.SchemaVersion = AttemptSchemaVersion
	}
	m.Operation = safeMetadataString(m.Operation, 96)
	m.Role = safeMetadataString(m.Role, 48)
	m.RequestedModel = safeMetadataString(m.RequestedModel, 160)
	m.EffectiveModel = safeMetadataString(m.EffectiveModel, 160)
	m.ItemName = safeMetadataString(m.ItemName, 160)
	m.ArtifactPath = safeMetadataString(m.ArtifactPath, 512)
	m.ErrorClass = safeMetadataString(m.ErrorClass, 64)
	m.RequestID = safeMetadataString(m.RequestID, 256)
	m.CorrelationID = safeMetadataString(m.CorrelationID, 128)
	return m
}

func safeMetadataString(value string, max int) string {
	value = strings.TrimSpace(value)
	value = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, value)
	if len(value) > max {
		return value[:max]
	}
	return value
}

// ErrorClass classifica uma falha sem guardar sua mensagem ou corpo remoto.
func ErrorClass(err error) string {
	if err == nil {
		return ""
	}
	var unknown *UnknownOutcomeError
	if errors.As(err, &unknown) {
		return "unknown_outcome"
	}
	var contract *ContractOutputError
	if errors.As(err, &contract) {
		return "contract"
	}
	var limit *ResourceLimitError
	if errors.As(err, &limit) {
		return "resource_limit"
	}
	if errors.Is(err, context.Canceled) {
		return "canceled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "deadline"
	}
	var retry *retryableError
	if errors.As(err, &retry) {
		return "transient"
	}
	return "permanent"
}

// AttemptCollector agrega eventos de workers concorrentes em ordem estável.
type AttemptCollector struct {
	mu     sync.Mutex
	next   int64
	events []AttemptMetadata
}

type attemptWriterContextKey struct{}

func WithAttemptWriter(ctx context.Context, writer func(AttemptMetadata)) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, attemptWriterContextKey{}, writer)
}

func attemptWriterFromContext(ctx context.Context) func(AttemptMetadata) {
	if ctx == nil {
		return nil
	}
	writer, _ := ctx.Value(attemptWriterContextKey{}).(func(AttemptMetadata))
	return writer
}

func NewAttemptCollector() *AttemptCollector { return &AttemptCollector{} }

func (c *AttemptCollector) Add(event AttemptMetadata) {
	if c == nil {
		return
	}
	event = event.Redacted()
	c.mu.Lock()
	c.next++
	event.Sequence = c.next
	c.events = append(c.events, event)
	c.mu.Unlock()
}

func (c *AttemptCollector) Snapshot() []AttemptMetadata {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	events := append([]AttemptMetadata(nil), c.events...)
	sort.SliceStable(events, func(i, j int) bool {
		if events[i].ItemIndex != events[j].ItemIndex {
			if events[i].ItemIndex == 0 {
				return false
			}
			if events[j].ItemIndex == 0 {
				return true
			}
			return events[i].ItemIndex < events[j].ItemIndex
		}
		if events[i].Attempt != events[j].Attempt {
			return events[i].Attempt < events[j].Attempt
		}
		return events[i].Sequence < events[j].Sequence
	})
	return events
}

// AnnotateItem associa o artefato publicado a todos os eventos daquele item.
func (c *AttemptCollector) AnnotateItem(itemIndex int, artifactPath string, reused bool) {
	if c == nil || itemIndex <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for index := range c.events {
		if c.events[index].ItemIndex != itemIndex {
			continue
		}
		c.events[index].ArtifactPath = safeMetadataString(artifactPath, 512)
		c.events[index].Reused = reused
	}
}

func (c *AttemptCollector) Writer() func(AttemptMetadata) {
	if c == nil {
		return nil
	}
	return c.Add
}

func fallbackOrdinal(model string, candidates []string) int {
	for index, candidate := range candidates {
		if candidate == model {
			return index
		}
	}
	return 0
}
