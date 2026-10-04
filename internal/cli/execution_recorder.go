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
			_, _ = recorder.finish(cmd.Context(), output.StateFailed, nil, err)
			return err
		}
		_, _ = recorder.finish(cmd.Context(), output.StateSuccess, nil, nil)
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
	if recorder != nil || openRequested(cmd) {
		options.Observe = func(result *output.Result) {
			persisted := recorder == nil
			warning := ""
			if recorder != nil {
				warning, persisted = recorder.finish(cmd.Context(), result.Status, result, nil)
			}
			if warning != "" {
				result.Warnings = append(result.Warnings, warning)
				if result.Status == output.StateSuccess {
					result.Status = output.StateWarning
				}
			}
			if persisted && openRequested(cmd) && (result.Status == output.StateSuccess || result.Status == output.StateWarning) {
				if warning := openPublishedResult(result); warning != "" {
					result.Warnings = append(result.Warnings, warning)
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

func (r *cliExecutionRecorder) finish(ctx context.Context, state output.State, result *output.Result, runErr error) (string, bool) {
	if r == nil {
		return "", true
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.finished {
		return "", true
	}
	r.finished = true
	if r.startErr != nil {
		return fmt.Sprintf("não foi possível registrar a execução no vault: %v", r.startErr), false
	}
	if r.store == nil {
		return "", false
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
	var outputs []string
	var primary string
	var shape vault.PathResultShape
	if result != nil {
		outputs = result.Outputs
		primary, shape = describePublishedResult(result)
		if primary != "" {
			outputs = append(append([]string(nil), outputs...), primary)
		}
	}
	paths := publishedRunPaths(outputs)
	if err := r.store.FinishPathRunResult(ctx, r.runID, status, r.inputs, paths, primary, shape, runErr); err != nil {
		return fmt.Sprintf("não foi possível concluir o registro da execução: %v", err), false
	}
	if len(paths) > 0 {
		if report, err := r.store.Sync(ctx, false); err != nil {
			return fmt.Sprintf("outputs publicados, mas o índice não pôde ser atualizado: %v", err), true
		} else if len(report.Warnings) > 0 {
			return "outputs publicados; uma fonte do índice não pôde ser atualizada", true
		}
	}
	return "", true
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
