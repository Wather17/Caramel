package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"caramel/internal/output"
	"caramel/internal/vault"

	"github.com/spf13/cobra"
)

const domainExecutionAnnotation = "caramel.io/domain-execution"

type executionContextKey struct{}

type cliExecutionRecorder struct {
	mu       sync.Mutex
	store    *vault.Vault
	runID    string
	inputs   []string
	startErr error
	finished bool
}

func registerDomainExecution(cmd *cobra.Command) {
	if cmd.Annotations == nil {
		cmd.Annotations = make(map[string]string)
	}
	if cmd.Annotations[domainExecutionAnnotation] == "true" {
		return
	}
	cmd.Annotations[domainExecutionAnnotation] = "true"
	original := cmd.RunE
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		err := original(cmd, args)
		recorder := executionRecorderFrom(cmd)
		if recorder == nil {
			return err
		}
		if err != nil {
			_ = recorder.finish(cmd.Context(), output.StateFailed, nil, err)
			return err
		}
		_ = recorder.finish(cmd.Context(), output.StateSuccess, nil, nil)
		return nil
	}
}

func startCLIExecution(cmd *cobra.Command, args []string) {
	if cmd.Annotations[domainExecutionAnnotation] != "true" {
		return
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	inputs := append([]string(nil), args...)
	if cmd == imageGenerateCmd {
		if value, err := cmd.Flags().GetString("file"); err == nil && strings.TrimSpace(value) != "" {
			inputs = append(inputs, value)
		}
	}
	inputs = existingRunPaths(inputs)
	recorder := &cliExecutionRecorder{inputs: inputs}
	store, err := vault.Open()
	if err != nil {
		recorder.startErr = err
	} else {
		run, startErr := store.StartPathRun(ctx, cmd.CommandPath(), map[string]string{"output_mode": string(outputOptions().Mode())}, inputs)
		if startErr != nil {
			recorder.startErr = startErr
			_ = store.Close()
		} else {
			recorder.store = store
			recorder.runID = run.ID
		}
	}
	cmd.SetContext(context.WithValue(ctx, executionContextKey{}, recorder))
}

func outputOptionsFor(cmd *cobra.Command) output.Options {
	options := outputOptions()
	recorder := executionRecorderFrom(cmd)
	if recorder != nil {
		options.Observe = func(result *output.Result) {
			if warning := recorder.finish(cmd.Context(), result.Status, result.Outputs, nil); warning != "" {
				result.Warnings = append(result.Warnings, warning)
				if result.Status == output.StateSuccess {
					result.Status = output.StateWarning
				}
			}
		}
	}
	return options
}

func executionRecorderFrom(cmd *cobra.Command) *cliExecutionRecorder {
	if cmd == nil || cmd.Context() == nil {
		return nil
	}
	recorder, _ := cmd.Context().Value(executionContextKey{}).(*cliExecutionRecorder)
	return recorder
}

func (r *cliExecutionRecorder) finish(ctx context.Context, state output.State, outputs []string, runErr error) string {
	if r == nil {
		return ""
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.finished {
		return ""
	}
	r.finished = true
	if r.startErr != nil {
		return fmt.Sprintf("não foi possível registrar a execução no vault: %v", r.startErr)
	}
	if r.store == nil {
		return ""
	}
	defer r.store.Close()
	if ctx == nil {
		ctx = context.Background()
	}
	status := "completed"
	if runErr != nil {
		status = "failed"
	} else if state != "" && state != output.StateSuccess {
		status = string(state)
	}
	paths := publishedRunPaths(outputs)
	if err := r.store.FinishPathRun(ctx, r.runID, status, r.inputs, paths, runErr); err != nil {
		return fmt.Sprintf("não foi possível concluir o registro da execução: %v", err)
	}
	if len(paths) > 0 {
		if report, err := r.store.Sync(ctx, false); err != nil {
			return fmt.Sprintf("outputs publicados, mas o índice não pôde ser atualizado: %v", err)
		} else if len(report.Warnings) > 0 {
			return "outputs publicados; uma fonte do índice não pôde ser atualizada"
		}
	}
	return ""
}

func existingRunPaths(paths []string) []string {
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil || (!info.Mode().IsRegular() && !info.IsDir()) {
			continue
		}
		if absolute, err := filepath.Abs(path); err == nil {
			result = append(result, filepath.Clean(absolute))
		}
	}
	return result
}

func publishedRunPaths(outputs []string) []string {
	seen := make(map[string]bool)
	var result []string
	add := func(path string) {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return
		}
		absolute = filepath.Clean(absolute)
		if !seen[absolute] {
			seen[absolute] = true
			result = append(result, absolute)
		}
	}
	for _, path := range outputs {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		add(path)
		if !info.IsDir() {
			continue
		}
		_ = filepath.WalkDir(path, func(child string, entry os.DirEntry, walkErr error) error {
			if walkErr == nil && !entry.IsDir() {
				add(child)
			}
			return nil
		})
	}
	return result
}
