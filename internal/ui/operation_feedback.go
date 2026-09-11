package ui

import (
	"context"
	"errors"
	"fmt"

	"caramel/internal/output"
	"caramel/internal/workflow"
)

// workflowEventState normaliza eventos antigos, que só preenchiam Step, para
// os estados compartilhados sem impor o layout da CLI à interface visual.
func workflowEventState(event workflow.ProgressEvent) output.State {
	if event.State != "" {
		return event.State
	}
	switch event.Step {
	case "skipped":
		return output.StateSkipped
	case "done", "saved":
		return output.StateSuccess
	case "error":
		return output.StateFailed
	case "canceled", "cancelled":
		return output.StateCanceled
	default:
		return ""
	}
}

func formatWorkflowEvent(event workflow.ProgressEvent) string {
	progress := ""
	if event.Total > 0 {
		progress = fmt.Sprintf(" [%d/%d]", event.Current, event.Total)
	}
	switch workflowEventState(event) {
	case output.StateSkipped:
		return fmt.Sprintf("↷ Ignorado%s: %s", progress, event.Message)
	case output.StateSuccess:
		if event.Step == "done" {
			return fmt.Sprintf("✓ Concluído%s: %s", progress, event.Message)
		}
		return fmt.Sprintf("✓ Salvo%s: %s", progress, event.Message)
	case output.StateFailed:
		return fmt.Sprintf("✗ Falha%s: %s", progress, event.Message)
	case output.StateCanceled:
		return fmt.Sprintf("■ Cancelado%s: %s", progress, event.Message)
	default:
		return fmt.Sprintf("… Em andamento%s: %s", progress, event.Message)
	}
}

func formatWorkflowCompletion(count int, err error) string {
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return "■ Operação cancelada."
		}
		return fmt.Sprintf("✗ Operação falhou: %v", err)
	}
	if count == 0 {
		return "⚠ Operação concluída sem resultados."
	}
	return fmt.Sprintf("✓ Operação concluída: %d resultado(s) criado(s).", count)
}
