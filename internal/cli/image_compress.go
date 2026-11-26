package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"caramel/internal/output"
	"caramel/internal/tools/compress"
	"caramel/internal/tools/pdf"
	"github.com/spf13/cobra"
)

var (
	icPreset                              string
	icQuality, icMaxDim                   int
	icKeep, icRecursive, icInPlace, icDry bool
	icMin, icOutput                       string
)
var imageCompressCmd = &cobra.Command{Use: "compress <imagem|pasta> [outros...]", Aliases: []string{"comprimir", "opt"}, Short: "Comprime imagens em lote", Long: "Comprime imagens individuais ou em lote.\n\n📚 QUANDO USAR:\nUse para reduzir o tamanho de imagens antes de compartilhar ou arquivar.", Example: "caramel image compress foto.png\ncaramel image compress ./materiais --recursive --preset balanced", Args: cobra.MinimumNArgs(1), RunE: runImageCompress}

func runImageCompress(cmd *cobra.Command, args []string) error {
	if icInPlace && icOutput != "" {
		return fmt.Errorf("--in-place não pode ser combinado com --output")
	}
	opts, e := compress.ResolveOptions(compress.Options{Preset: compress.Preset(icPreset), Quality: icQuality, MaxDim: icMaxDim, MaxDimSet: cmd.Flags().Changed("max-dim"), KeepFormat: icKeep})
	if e != nil {
		return e
	}
	min, e := compress.ParseSizeInBytes(icMin)
	if e != nil {
		return e
	}
	var paths []string
	var sourceRoot string
	for _, arg := range args {
		info, e := os.Stat(arg)
		if e != nil {
			return e
		}
		if !info.IsDir() {
			if !compress.IsSupportedImageFile(arg) {
				return fmt.Errorf("formato não suportado: %s", arg)
			}
			paths = append(paths, arg)
			continue
		}
		if len(args) == 1 {
			sourceRoot = arg
		}
		if icRecursive {
			e = filepath.WalkDir(arg, func(p string, d os.DirEntry, e error) error {
				if e != nil {
					return e
				}
				if !d.IsDir() && compress.IsSupportedImageFile(p) {
					paths = append(paths, p)
				}
				return nil
			})
		} else {
			var es []os.DirEntry
			es, e = os.ReadDir(arg)
			if e == nil {
				for _, d := range es {
					p := filepath.Join(arg, d.Name())
					if !d.IsDir() && compress.IsSupportedImageFile(p) {
						paths = append(paths, p)
					}
				}
			}
		}
		if e != nil {
			return e
		}
	}
	pdf.SortNatural(paths)
	if len(paths) == 0 {
		return fmt.Errorf("nenhuma imagem suportada encontrada")
	}
	var plan *output.Plan
	if icOutput == "" && !icInPlace && !icDry {
		shape := output.ShapeBundle
		if len(paths) == 1 {
			shape = output.ShapeSingle
		}
		plan, e = libraryOutputPlan(cmd, "imagens", shape, outputStem(args[0], "_compressed"), "")
		if e != nil {
			return e
		}
		if plan != nil {
			defer plan.Cleanup()
		}
	}
	renderer, e := output.New(outputOptionsFor(cmd), cmd.OutOrStdout(), cmd.ErrOrStderr())
	if e != nil {
		return e
	}
	var outputs, warnings []string
	var items []compress.FileResult
	var before, after int64
	ok, skip, fail := 0, 0, 0
	for _, p := range paths {
		info, e := os.Stat(p)
		if e != nil {
			fail++
			continue
		}
		before += info.Size()
		if info.Size() < min {
			skip++
			items = append(items, compress.FileResult{Input: p, State: "skipped", Reason: "abaixo de --min-size", OriginalBytes: info.Size()})
			continue
		}
		b, e := os.ReadFile(p)
		if e != nil {
			fail++
			items = append(items, compress.FileResult{Input: p, State: "failed", Reason: e.Error(), OriginalBytes: info.Size()})
			continue
		}
		r, e := compress.CompressImageBytes(b, opts)
		if e != nil {
			fail++
			warnings = append(warnings, e.Error())
			continue
		}
		if r.Skipped {
			skip++
			items = append(items, compress.FileResult{Input: p, State: "skipped", Reason: r.Reason, OriginalBytes: info.Size()})
			continue
		}
		after += int64(len(r.Data))
		if icDry {
			ok++
			items = append(items, compress.FileResult{Input: p, State: "compressed", OriginalBytes: info.Size(), CompressedBytes: int64(len(r.Data)), Format: r.Format})
			continue
		}
		target := icOutput
		if plan != nil {
			if len(paths) == 1 {
				target = filepath.Join(plan.StageDir(), filepath.Base(p))
			} else {
				target = filepath.Join(plan.WorkPath(), filepath.Base(p))
			}
		}
		rel := filepath.Base(p)
		if sourceRoot != "" {
			if candidate, e := filepath.Rel(sourceRoot, p); e == nil {
				rel = candidate
			}
		}
		if target != "" && len(paths) > 1 {
			target = filepath.Join(target, rel)
		} else if target == "" && !icInPlace {
			if sourceRoot != "" {
				target = filepath.Join(sourceRoot+"_compressed", strings.TrimSuffix(rel, filepath.Ext(rel))+"_compressed"+r.Extension)
			} else {
				target = filepath.Join(filepath.Dir(p), strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))+"_compressed"+r.Extension)
			}
		} else if target == "" {
			target = p
		}
		if icInPlace {
			fr, e := compress.CompressImageFileInPlace(p, opts)
			if e != nil {
				fail++
				warnings = append(warnings, e.Error())
				continue
			}
			ok++
			outputs = append(outputs, fr.Output)
			items = append(items, fr)
			continue
		}
		e = compress.WriteImageBytes(target, r.Data)
		if e != nil {
			fail++
			warnings = append(warnings, e.Error())
			items = append(items, compress.FileResult{Input: p, State: "failed", Reason: e.Error(), OriginalBytes: info.Size()})
			continue
		}
		if icInPlace && target != p {
			_ = os.Remove(p)
		}
		ok++
		outputs = append(outputs, target)
		items = append(items, compress.FileResult{Input: p, Output: target, State: "compressed", OriginalBytes: int64(len(b)), CompressedBytes: int64(len(r.Data))})
	}
	if plan != nil && len(outputs) > 0 {
		outputs, e = publishOutputPlan(plan, outputs)
		if e != nil {
			return e
		}
	}
	status := output.StateSuccess
	if skip+fail > 0 {
		status = output.StateWarning
	}
	return renderer.Result(output.Result{Status: status, Summary: fmt.Sprintf("Compressão concluída: %d comprimida(s), %d pulada(s), %d falha(s).", ok, skip, fail), Count: ok, Data: map[string]any{"original_bytes": before, "compressed_bytes": after, "items": items}, Outputs: outputs, PrimaryOutput: firstOutput(outputs), Warnings: warnings})
}
func firstOutput(v []string) string {
	if len(v) > 0 {
		return v[0]
	}
	return ""
}
func init() {
	imageCompressCmd.Flags().StringVar(&icPreset, "preset", "balanced", "Preset light, balanced ou high")
	imageCompressCmd.Flags().IntVarP(&icQuality, "quality", "q", 0, "Qualidade JPEG")
	imageCompressCmd.Flags().IntVar(&icMaxDim, "max-dim", 0, "Dimensão máxima; 0 usa o preset")
	imageCompressCmd.Flags().BoolVar(&icKeep, "keep-format", false, "Preserva o formato original quando possível")
	imageCompressCmd.Flags().StringVarP(&icMin, "min-size", "s", "0", "Tamanho mínimo")
	imageCompressCmd.Flags().BoolVarP(&icRecursive, "recursive", "r", false, "Inclui subpastas")
	imageCompressCmd.Flags().StringVarP(&icOutput, "output", "o", "", "Destino")
	imageCompressCmd.Flags().BoolVar(&icInPlace, "in-place", false, "Substitui originais")
	imageCompressCmd.Flags().BoolVar(&icDry, "dry-run", false, "Simula sem gravar")
	imageCmd.AddCommand(imageCompressCmd)
}
