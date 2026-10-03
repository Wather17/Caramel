package cli

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"caramel/internal/config"
	"caramel/internal/output"

	"github.com/spf13/cobra"
)

type commandStartedContextKey struct{}

func markCommandStarted(cmd *cobra.Command) {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	cmd.SetContext(context.WithValue(ctx, commandStartedContextKey{}, time.Now()))
}

func libraryOutputPlan(cmd *cobra.Command, category string, shape output.Shape, baseName, extension string) (*output.Plan, error) {
	var ctx context.Context
	if cmd != nil {
		ctx = cmd.Context()
	}
	return libraryOutputPlanContext(ctx, category, shape, baseName, extension)
}

func libraryOutputPlanContext(ctx context.Context, category string, shape output.Shape, baseName, extension string) (*output.Plan, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.LibraryDir) == "" {
		return nil, nil
	}
	root, err := config.ResolveLibraryDir(cfg)
	if err != nil {
		return nil, err
	}
	var startedAt time.Time
	if ctx != nil {
		startedAt, _ = ctx.Value(commandStartedContextKey{}).(time.Time)
	}
	return output.NewPlan(output.PlanRequest{
		LibraryRoot: root,
		Category:    category,
		Shape:       shape,
		BaseName:    baseName,
		Extension:   extension,
		StartedAt:   startedAt,
	})
}

func outputStem(path, suffix string) string {
	base := filepath.Base(filepath.Clean(path))
	base = strings.TrimSuffix(base, filepath.Ext(base))
	return output.SanitizeName(base + suffix)
}

func publishOutputPlan(plan *output.Plan, produced []string) ([]string, error) {
	if plan == nil {
		return produced, nil
	}
	return plan.Publish(produced)
}

func plannedPrimaryOutput(plan *output.Plan) string {
	if plan == nil {
		return ""
	}
	return plan.FinalPath()
}

func plannedPrimaryOutputFor(plan *output.Plan, outputs []string) string {
	if len(outputs) == 0 {
		return ""
	}
	return plannedPrimaryOutput(plan)
}

func bundlePrimaryOutput(plan *output.Plan, outputs []string) string {
	if primary := plannedPrimaryOutputFor(plan, outputs); primary != "" {
		return primary
	}
	if len(outputs) == 0 {
		return ""
	}
	return filepath.Dir(outputs[0])
}
