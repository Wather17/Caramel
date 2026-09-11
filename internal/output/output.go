// Package output define a semântica comum para resultados de operações da CLI.
package output

import (
	"encoding/json"
	"fmt"
	"io"
)

// Mode identifica o formato principal de saída de uma execução.
type Mode string

const (
	ModeHuman Mode = "human"
	ModeQuiet Mode = "quiet"
	ModeJSON  Mode = "json"
)

// Options controla a apresentação de uma execução.
type Options struct {
	Verbose bool
	Quiet   bool
	JSON    bool
}

// Validate rejeita combinações de modos que não possuem semântica inequívoca.
func (o Options) Validate() error {
	if o.Quiet && o.JSON {
		return fmt.Errorf("as flags --quiet e --json não podem ser usadas juntas")
	}
	return nil
}

// Mode retorna o modo efetivo da saída.
func (o Options) Mode() Mode {
	if o.JSON {
		return ModeJSON
	}
	if o.Quiet {
		return ModeQuiet
	}
	return ModeHuman
}

// State descreve o resultado observável de uma operação.
type State string

const (
	StateSuccess  State = "success"
	StateWarning  State = "warning"
	StateFailed   State = "failed"
	StateSkipped  State = "skipped"
	StateCanceled State = "canceled"
)

// Event representa progresso sem impor um layout à CLI ou à TUI.
type Event struct {
	State   State  `json:"state,omitempty"`
	Step    string `json:"step,omitempty"`
	Current int    `json:"current,omitempty"`
	Total   int    `json:"total,omitempty"`
	Message string `json:"message,omitempty"`
	Path    string `json:"path,omitempty"`
}

// Result contém o resumo serializável de uma operação.
type Result struct {
	Status   State       `json:"status"`
	Summary  string      `json:"summary,omitempty"`
	Count    int         `json:"count,omitempty"`
	Data     interface{} `json:"data,omitempty"`
	Outputs  []string    `json:"outputs,omitempty"`
	Warnings []string    `json:"warnings,omitempty"`
	Errors   []string    `json:"errors,omitempty"`
}

// Renderer separa resultado, progresso e diagnóstico nos canais apropriados.
type Renderer struct {
	options Options
	out     io.Writer
	err     io.Writer
}

// New cria um renderer com writers explícitos. Nil writers são descartados.
func New(options Options, out, err io.Writer) (*Renderer, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	if out == nil {
		out = io.Discard
	}
	if err == nil {
		err = io.Discard
	}
	return &Renderer{options: options, out: out, err: err}, nil
}

// Options retorna uma cópia das opções do renderer.
func (r *Renderer) Options() Options {
	return r.options
}

// Progress emite progresso somente quando solicitado no modo verbose.
// No modo JSON, o stdout permanece reservado para um único resultado.
func (r *Renderer) Progress(event Event) {
	if r == nil || r.options.Quiet || !r.options.Verbose {
		return
	}
	if r.options.JSON {
		_, _ = fmt.Fprintf(r.err, "[%d/%d] %s\n", event.Current, event.Total, event.Message)
		return
	}
	_, _ = fmt.Fprintf(r.err, "[%s] %s\n", event.Step, event.Message)
}

// Diagnostic escreve detalhes técnicos apenas no modo verbose.
func (r *Renderer) Diagnostic(format string, args ...interface{}) {
	if r == nil || r.options.Quiet || !r.options.Verbose {
		return
	}
	_, _ = fmt.Fprintf(r.err, format, args...)
}

// Text escreve texto humano opcional. Em quiet e JSON, o stdout permanece reservado.
func (r *Renderer) Text(format string, args ...interface{}) error {
	if r == nil {
		return fmt.Errorf("renderer de output não inicializado")
	}
	if r.options.Quiet || r.options.JSON {
		return nil
	}
	_, err := fmt.Fprintf(r.out, format, args...)
	return err
}

// Result escreve o resumo no formato selecionado.
func (r *Renderer) Result(result Result) error {
	if r == nil {
		return fmt.Errorf("renderer de output não inicializado")
	}
	if r.options.JSON {
		return json.NewEncoder(r.out).Encode(result)
	}
	if r.options.Quiet {
		for _, failure := range result.Errors {
			if _, err := fmt.Fprintf(r.err, "Erro: %s\n", failure); err != nil {
				return err
			}
		}
		return nil
	}
	if result.Summary != "" {
		if _, err := fmt.Fprintln(r.out, result.Summary); err != nil {
			return err
		}
	}
	for _, path := range result.Outputs {
		if _, err := fmt.Fprintln(r.out, path); err != nil {
			return err
		}
	}
	for _, warning := range result.Warnings {
		if _, err := fmt.Fprintf(r.err, "Aviso: %s\n", warning); err != nil {
			return err
		}
	}
	for _, failure := range result.Errors {
		if _, err := fmt.Fprintf(r.err, "Erro: %s\n", failure); err != nil {
			return err
		}
	}
	return nil
}
