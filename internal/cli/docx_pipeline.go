package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"caramel/internal/config"
	"caramel/internal/tools/ai"
	"caramel/internal/tools/docx"
	"caramel/internal/tools/pdf"
	"caramel/internal/tools/pipeline"
	"caramel/internal/ui"
)

// ProcessDocxOptions contém os parâmetros para execução do pipeline de processamento e reconstrução de .docx
type ProcessDocxOptions struct {
	DocxPath    string
	OutputDir   string
	ModelName   string
	MinSize     string
	Interactive bool
	Verbose     bool
	TriageModel string // Modelo de visão usado na triagem (vazio = padrão gratuito)
	NoTriage    bool   // true desativa a triagem e colora todas as imagens elegíveis
}

// RunProcessDocx executa o fluxo completo do pipeline DOCX (interativo ou automatizado)
func RunProcessDocx(opts ProcessDocxOptions) error {
	docxPath := opts.DocxPath

	if !strings.HasSuffix(strings.ToLower(docxPath), ".docx") {
		return fmt.Errorf("o arquivo '%s' não possui a extensão .docx", docxPath)
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	if cfg.OpenRouterAPIKey == "" {
		return fmt.Errorf("chave de API do OpenRouter não configurada. Use 'caramel config setup' ou 'caramel config set openrouter_key <sua-chave>'")
	}

	modelName := opts.ModelName
	if modelName == "" {
		modelName = ai.DefaultModel
	}

	minSizeStr := opts.MinSize
	if minSizeStr == "" {
		minSizeStr = "0"
	}

	minSizeBytes, err := docx.ParseSizeInBytes(minSizeStr)
	if err != nil {
		return err
	}

	// Modo Interativo (--interactive / -i) com preview ANSI TrueColor no terminal
	if opts.Interactive {
		allImages, err := docx.ListImages(docxPath)
		if err != nil {
			return err
		}

		if len(allImages) == 0 {
			fmt.Printf("ℹ️  Nenhuma imagem foi encontrada no arquivo '%s'.\n", docxPath)
			return nil
		}

		keptImages, _ := docx.FilterImagesByMinSize(allImages, minSizeBytes)
		if len(keptImages) == 0 {
			fmt.Printf("ℹ️  Nenhuma imagem com tamanho >= %s foi encontrada em '%s'.\n", minSizeStr, docxPath)
			return nil
		}

		// Extrai imagens elegíveis para pasta temporária para gerar os previews ANSI
		tempExtractDir := filepath.Join(os.TempDir(), "caramel_process_preview_"+strings.TrimSuffix(filepath.Base(docxPath), filepath.Ext(docxPath)))
		defer os.RemoveAll(tempExtractDir)

		_, err = docx.ExtractImagesFromList(docxPath, tempExtractDir, keptImages)
		if err != nil {
			return fmt.Errorf("falha ao preparar imagens para preview: %w", err)
		}

		var candidatePaths []string
		imgMap := make(map[string]docx.ExtractedImage)
		for _, img := range keptImages {
			fullPath := filepath.Join(tempExtractDir, img.OriginalName)
			candidatePaths = append(candidatePaths, fullPath)
			imgMap[fullPath] = img
		}

		// Ordena numericamente (natural sort)
		pdf.SortNatural(candidatePaths)

		selectedPaths, err := ui.SelectImageFilesWithPreviewInteractive(candidatePaths)
		if err != nil {
			return err
		}

		if len(selectedPaths) == 0 {
			fmt.Println("ℹ️  Nenhuma imagem foi selecionada.")
			return nil
		}

		selectedImages := make([]docx.ExtractedImage, 0, len(selectedPaths))
		for _, p := range selectedPaths {
			if imgInfo, ok := imgMap[p]; ok {
				selectedImages = append(selectedImages, imgInfo)
			}
		}

		fmt.Printf("🚀 Iniciando Pipeline Automatizado para %d imagem(ns) selecionada(s)...\n", len(selectedImages))
		res, err := pipeline.RunDocxPipelineSelected(docxPath, opts.OutputDir, cfg.OpenRouterAPIKey, modelName, selectedImages, opts.Verbose, opts.TriageModel, opts.NoTriage)
		if err != nil {
			return err
		}

		printTriageSummary(res)

		fmt.Printf("✅ Pipeline concluído com sucesso!\n")
		fmt.Printf(" ├─ Total de imagens coloridas/substituídas: %d\n", res.TotalColorized)
		if res.RebuiltDocxPath != "" {
			fmt.Printf(" ├─ Novo arquivo reconstruído: %s\n", res.RebuiltDocxPath)
		}
		fmt.Printf(" └─ Imagens individuais salvas no diretório: %s\n", res.OutputDir)

		return nil
	}

	// Execução Automatizada Padrão (Colora todas as imagens mantidas pelo filtro minSize)
	fmt.Printf("🚀 Iniciando Pipeline Automatizado para '%s'...\n", filepath.Base(docxPath))
	fmt.Printf(" ├─ Modelo IA: %s\n", modelName)
	if minSizeBytes > 0 {
		fmt.Printf(" ├─ Filtro de Tamanho Mínimo: %s (%d bytes)\n", minSizeStr, minSizeBytes)
	}

	res, err := pipeline.RunDocxPipeline(docxPath, opts.OutputDir, cfg.OpenRouterAPIKey, modelName, minSizeBytes, opts.Verbose, opts.TriageModel, opts.NoTriage)
	if err != nil {
		return err
	}

	if res.TotalSkipped > 0 {
		fmt.Printf(" ├─ Imagens ignoradas (tamanho < %s): %d\n", minSizeStr, res.TotalSkipped)
		for _, img := range res.SkippedImages {
			sizeKB := float64(img.Size) / 1024.0
			fmt.Printf(" │   └─ Ignorada: %s (%.1f KB)\n", img.OriginalName, sizeKB)
		}
	}

	printTriageSummary(res)

	if res.TotalColorized == 0 {
		if res.TotalTriageSkipped > 0 {
			fmt.Printf("ℹ️  Nenhuma imagem foi aprovada pela triagem em '%s' (todas foram puladas).\n", docxPath)
		} else {
			fmt.Printf("ℹ️  Nenhuma imagem com tamanho >= %s foi encontrada em '%s'.\n", minSizeStr, docxPath)
		}
		return nil
	}

	fmt.Printf("✅ Pipeline concluído com sucesso!\n")
	fmt.Printf(" ├─ Total de imagens coloridas/substituídas: %d\n", res.TotalColorized)
	if res.RebuiltDocxPath != "" {
		fmt.Printf(" ├─ Novo arquivo reconstruído: %s\n", res.RebuiltDocxPath)
	}
	fmt.Printf(" └─ Imagens individuais salvas no diretório: %s\n", res.OutputDir)

	return nil
}

// printTriageSummary exibe o resumo das imagens puladas pela triagem de economia, se houver
func printTriageSummary(res *pipeline.PipelineResult) {
	if res == nil || res.TotalTriageSkipped == 0 {
		return
	}

	fmt.Printf(" ├─ Imagens puladas pela triagem (economia de API): %d\n", res.TotalTriageSkipped)
	for _, skipped := range res.TriageSkipped {
		stage := "LLM"
		if skipped.Stage == "local" {
			stage = "análise local"
		}
		fmt.Printf(" │   └─ %s [%s]: %s\n", skipped.Name, stage, skipped.Reason)
	}
}
