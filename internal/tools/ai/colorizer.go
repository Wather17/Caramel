package ai

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"caramel/internal/prompts"
)

// ColorizeOptions contém todos os parâmetros para o processo de coloração de uma imagem
type ColorizeOptions struct {
	OutputDir        string    // Diretório onde a imagem colorida será salva
	APIKey           string    // Chave da API do OpenRouter
	Model            string    // Modelo de geração de imagem (padrão: DefaultModel)
	TriageModel      string    // Modelo de visão para a triagem (padrão: DefaultTriageModel)
	DisableTriage    bool      // true desativa as duas camadas de triagem (coloração forçada)
	MaxWorkers       int       // Número máximo de imagens processadas em paralelo (0 = adaptativo)
	Verbose          bool      // Exibe logs detalhados de depuração
	DiagnosticWriter io.Writer // Canal para diagnóstico verbose
}

// ColorizeResult contém o relatório do processo de coloração
type ColorizeResult struct {
	OriginalPath  string
	ColorizedPath string

	// Campos de triagem: preenchidos quando a imagem é pulada antes da coloração
	Skipped    bool   // true se a imagem foi rejeitada pela triagem (não foi colorida)
	SkipStage  string // "local" (análise de saturação) ou "triage" (LLM de visão)
	SkipReason string // Motivo legível da rejeição
}

// ColorizeBatchResult guarda o resultado indexado de uma imagem processada em lote.
type ColorizeBatchResult struct {
	Path   string
	Result *ColorizeResult
	Err    error
}

// ColorizeImages executa colorização em lote usando um contexto de fundo.
func ColorizeImages(imagePaths []string, opts ColorizeOptions, onProgress BatchProgressFunc) ([]ColorizeBatchResult, error) {
	return ColorizeImagesContext(context.Background(), imagePaths, opts, onProgress)
}

// ColorizeImagesContext processa cada imagem completa dentro de um worker e
// preserva os resultados na ordem original dos caminhos recebidos.
func ColorizeImagesContext(ctx context.Context, imagePaths []string, opts ColorizeOptions, onProgress BatchProgressFunc) ([]ColorizeBatchResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	results := make([]ColorizeBatchResult, len(imagePaths))
	itemErrors, batchErr := ExecuteBatchContext(ctx, len(imagePaths), opts.MaxWorkers, func(workCtx context.Context, index int) error {
		path := imagePaths[index]
		result, err := ColorizeSingleImageContext(workCtx, path, opts)
		results[index] = ColorizeBatchResult{Path: path, Result: result, Err: err}
		return err
	}, onProgress)
	for index, err := range itemErrors {
		if results[index].Err == nil && err != nil {
			results[index].Err = err
		}
	}
	return results, batchErr
}

// ColorizeSingleImage recebe o caminho de uma imagem, executa a triagem de economia
// (análise local de saturação + LLM de visão barata) e, se aprovada, envia para a IA
// de coloração e salva a versão colorida.
//
// A triagem é fail-open: qualquer erro na análise local ou na chamada da LLM de triagem
// faz a imagem seguir normalmente para a coloração, garantindo que nenhuma ilustração
// legítima seja perdida por instabilidade da camada de economia.
func ColorizeSingleImage(imagePath string, opts ColorizeOptions) (*ColorizeResult, error) {
	return ColorizeSingleImageContext(context.Background(), imagePath, opts)
}

// ColorizeSingleImageContext é a variante cancelável de ColorizeSingleImage.
func ColorizeSingleImageContext(ctx context.Context, imagePath string, opts ColorizeOptions) (*ColorizeResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	client, err := NewClient(opts.APIKey)
	if err != nil {
		return nil, err
	}
	client.Verbose = opts.Verbose
	if opts.DiagnosticWriter != nil {
		client.DiagnosticWriter = opts.DiagnosticWriter
	}

	// Triagem de economia: evita gastar a API de geração com imagens que não precisam de cor
	if !opts.DisableTriage {
		if skipped, result := checkTriageContext(ctx, imagePath, client, opts); skipped {
			return result, nil
		}
	}

	prompt := prompts.GetColorizationPrompt()

	// Coloração com retry: falhas transitórias (429, 5xx, rede) são reexecutadas
	// com backoff para não perder a imagem no batch/docx.
	var imgBytes []byte
	var ext string
	colorizeErr := retryWithBackoffContext(ctx, 3, func() error {
		var e error
		imgBytes, ext, e = client.ColorizeImageContext(ctx, imagePath, prompt, opts.Model)
		return e
	})
	if colorizeErr != nil {
		return nil, fmt.Errorf("falha ao colorir imagem '%s': %w", filepath.Base(imagePath), colorizeErr)
	}

	if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("falha ao criar pasta de saída '%s': %w", opts.OutputDir, err)
	}

	baseName := strings.TrimSuffix(filepath.Base(imagePath), filepath.Ext(imagePath))
	outFileName := fmt.Sprintf("%s_colorida.%s", baseName, ext)
	outputPath := filepath.Join(opts.OutputDir, outFileName)

	if err := os.WriteFile(outputPath, imgBytes, 0644); err != nil {
		return nil, fmt.Errorf("falha ao salvar imagem colorida no disco '%s': %w", outputPath, err)
	}

	return &ColorizeResult{
		OriginalPath:  imagePath,
		ColorizedPath: outputPath,
	}, nil
}

// checkTriage executa as duas camadas de triagem de economia antes da coloração:
//
//  1. Camada local (custo zero): análise de saturação HSV — detecta imagens já coloridas;
//  2. Camada LLM (custo irrisório ou grátis): modelo de visão decide se a imagem em P&B
//     é uma ilustração colorível ou apenas texto/tabela/diagrama.
//
// Retorna skipped=true e um ColorizeResult preenchido quando a imagem deve ser pulada.
// Em qualquer erro de análise, adota fail-open (skipped=false) e loga em modo verbose.
func checkTriage(imagePath string, client *Client, opts ColorizeOptions) (bool, *ColorizeResult) {
	return checkTriageContext(context.Background(), imagePath, client, opts)
}

func checkTriageContext(ctx context.Context, imagePath string, client *Client, opts ColorizeOptions) (bool, *ColorizeResult) {
	if ctx == nil {
		ctx = context.Background()
	}
	baseName := filepath.Base(imagePath)

	// Camada 1: análise local de saturação (sem custo de API)
	colored, ratio, err := IsLikelyAlreadyColored(imagePath)
	if err != nil {
		// Falha na decodificação local (ex: SVG): segue para a camada LLM decidir
		client.debugf("⚠️ [TRIAGE] Análise local indisponível para '%s': %v (seguindo para LLM)\n", baseName, err)
	} else if colored {
		return true, &ColorizeResult{
			OriginalPath: imagePath,
			Skipped:      true,
			SkipStage:    "local",
			SkipReason:   fmt.Sprintf("imagem já colorida (%.0f%% dos pixels com saturação)", ratio*100),
		}
	}

	// Camada 2: triagem por LLM de visão barata (com retry em falhas transitórias)
	triageModel := opts.TriageModel
	if triageModel == "" {
		triageModel = DefaultTriageModel
	}

	client.debugf("🔎 [TRIAGE] Analisando '%s' com o modelo '%s'...\n", baseName, triageModel)

	var triageRes *TriageResult
	triageErr := retryWithBackoffContext(ctx, 2, func() error {
		var e error
		triageRes, e = client.TriageImageContext(ctx, imagePath, prompts.GetTriagePrompt(), triageModel)
		return e
	})
	if triageErr != nil {
		// Fail-open: em caso de erro (rate limit, API fora, parse), colore mesmo assim
		client.debugf("⚠️ [TRIAGE] Falha na triagem de '%s': %v (fail-open: colorindo mesmo assim)\n", baseName, triageErr)
		return false, nil
	}

	if !triageRes.ShouldColorize {
		return true, &ColorizeResult{
			OriginalPath: imagePath,
			Skipped:      true,
			SkipStage:    "triage",
			SkipReason:   triageRes.Reason,
		}
	}

	return false, nil
}
