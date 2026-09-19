package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// StructuredJSONKind identifica o formato estruturado exigido por uma operação.
type StructuredJSONKind string

const (
	StructuredJSONArray  StructuredJSONKind = "array JSON"
	StructuredJSONObject StructuredJSONKind = "objeto JSON"
)

// ContractOutputError indica que o modelo respondeu fora do contrato de saída.
// O trecho da resposta fica disponível somente para diagnóstico verbose.
type ContractOutputError struct {
	Operation string
	Model     string
	Expected  StructuredJSONKind
	Reason    string
	Snippet   string
}

func (e *ContractOutputError) Error() string {
	operation := e.Operation
	if operation == "" {
		operation = "operação de IA"
	}
	model := e.Model
	if model == "" {
		model = "desconhecido"
	}
	expected := string(e.Expected)
	if expected == "" {
		expected = "JSON válido"
	}
	if e.Reason == "" {
		return fmt.Sprintf("falha de contrato na %s (modelo %s): esperado %s", operation, model, expected)
	}
	return fmt.Sprintf("falha de contrato na %s (modelo %s): esperado %s: %s", operation, model, expected, e.Reason)
}

// Diagnostic retorna uma explicação verbose com trecho limitado da resposta.
func (e *ContractOutputError) Diagnostic() string {
	if e == nil || e.Snippet == "" {
		return ""
	}
	return fmt.Sprintf("resposta recebida (trecho truncado): %q", e.Snippet)
}

// ParseStructuredArray extrai e valida a presença de um array JSON na resposta.
// Texto incidental e cercas Markdown são tolerados quando existe exatamente um
// array válido; múltiplos valores estruturados são rejeitados como ambíguos.
func ParseStructuredArray(raw, operation, model string) (json.RawMessage, error) {
	return ParseStructuredJSON(raw, StructuredJSONArray, operation, model)
}

// ParseStructuredObject extrai e valida a presença de um objeto JSON na resposta.
func ParseStructuredObject(raw, operation, model string) (json.RawMessage, error) {
	return ParseStructuredJSON(raw, StructuredJSONObject, operation, model)
}

// ParseStructuredJSON extrai um único valor JSON do tipo solicitado.
func ParseStructuredJSON(raw string, kind StructuredJSONKind, operation, model string) (json.RawMessage, error) {
	return parseStructuredJSON(raw, kind, operation, model)
}

// NewContractOutputError cria um erro de contrato para consumidores que fazem
// a validação específica do schema fora deste pacote.
func NewContractOutputError(operation, model string, expected StructuredJSONKind, raw, reason string) error {
	return newContractOutputError(operation, model, expected, raw, reason)
}

func parseStructuredJSON(raw string, kind StructuredJSONKind, operation, model string) (json.RawMessage, error) {
	cleaned := strings.TrimSpace(strings.TrimPrefix(raw, "\ufeff"))
	if cleaned == "" {
		return nil, newContractOutputError(operation, model, kind, raw, "resposta vazia")
	}

	wantStart := byte('[')
	if kind == StructuredJSONObject {
		wantStart = '{'
	}
	start := -1
	for i := 0; i < len(cleaned); i++ {
		if cleaned[i] == '[' || cleaned[i] == '{' {
			start = i
			break
		}
	}
	if start == -1 {
		return nil, newContractOutputError(operation, model, kind, raw, "nenhum valor estruturado encontrado")
	}
	if cleaned[start] != wantStart {
		return nil, newContractOutputError(operation, model, kind, raw, fmt.Sprintf("foi encontrado %s", describeStructuredStart(cleaned[start])))
	}

	value, end, err := decodeStructuredCandidate(cleaned[start:])
	if err != nil {
		return nil, newContractOutputError(operation, model, kind, raw, "JSON inválido ou truncado")
	}

	// Depois do primeiro valor completo, outro valor estruturado válido torna a
	// resposta ambígua e impede associar o conteúdo à operação com segurança.
	remaining := cleaned[start+end:]
	for i := 0; i < len(remaining); i++ {
		if remaining[i] != '[' && remaining[i] != '{' {
			continue
		}
		if _, _, probeErr := decodeStructuredCandidate(remaining[i:]); probeErr == nil {
			return nil, newContractOutputError(operation, model, kind, raw, "mais de um valor estruturado foi encontrado")
		}
	}

	return json.RawMessage(bytes.TrimSpace(value)), nil
}

func decodeStructuredCandidate(raw string) ([]byte, int, error) {
	decoder := json.NewDecoder(strings.NewReader(raw))
	var value json.RawMessage
	if err := decoder.Decode(&value); err != nil {
		return nil, 0, err
	}
	return value, int(decoder.InputOffset()), nil
}

func newContractOutputError(operation, model string, expected StructuredJSONKind, raw, reason string) *ContractOutputError {
	return &ContractOutputError{
		Operation: operation,
		Model:     model,
		Expected:  expected,
		Reason:    reason,
		Snippet:   truncateForError(raw),
	}
}

func describeStructuredStart(start byte) string {
	if start == '{' {
		return "um objeto JSON"
	}
	return "um array JSON"
}
