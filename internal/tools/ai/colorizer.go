package ai

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"caramel/internal/prompts"
)

// ColorizeOptions contém todos os parâmetros para o processo de coloração de uma imagem
type ColorizeOptions struct {
	OutputDir     string // Diretório onde a imagem colorida será salva
	APIKey        string // Chave da API do OpenRouter
	Model         string // Modelo de geração de imagem (padrão: DefaultModel)
	TriageModel   string // Modelo de visão para a triagem (padrão: DefaultTriageModel)
	DisableTriage bool   // true desativa as duas camadas de triagem (coloração forçada)
	Verbose       bool   // Exibe logs detalhados de depuração
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

// ColorizeSingleImage recebe o caminho de uma imagem, executa a triagem de economia
// (análise local de saturação + LLM de visão barata) e, se aprovada, envia para a IA
// de coloração e salva a versão colorida.
//
// A triagem é fail-open: qualquer erro na análise local ou na chamada da LLM de triagem
// faz a imagem seguir normalmente para a coloração, garantindo que nenhuma ilustração
// legítima seja perdida por instabilidade da camada de economia.
func ColorizeSingleImage(imagePath string, opts ColorizeOptions) (*ColorizeResult, error) {
	client, err := NewClient(opts.APIKey)
	if err != nil {
		return nil, err
	}
	client.Verbose = opts.Verbose

	// Triagem de economia: evita gastar a API de geração com imagens que não precisam de cor
	if !opts.DisableTriage {
		if skipped, result := checkTriage(imagePath, client, opts); skipped {
			return result, nil
		}
	}

	prompt := prompts.GetColorizationPrompt()

	// Coloração com retry: falhas transitórias (429, 5xx, rede) são reexecutadas
	// com backoff para não perder a imagem no batch/docx.
	var imgBytes []byte
	var ext string
	colorizeErr := retryWithBackoff(3, func() error {
		var e error
		imgBytes, ext, e = client.ColorizeImage(imagePath, prompt, opts.Model)
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
	baseName := filepath.Base(imagePath)

	// Camada 1: análise local de saturação (sem custo de API)
	colored, ratio, err := IsLikelyAlreadyColored(imagePath)
	if err != nil {
		// Falha na decodificação local (ex: SVG): segue para a camada LLM decidir
		if opts.Verbose {
			fmt.Printf("⚠️ [TRIAGE] Análise local indisponível para '%s': %v (seguindo para LLM)\n", baseName, err)
		}
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

	if opts.Verbose {
		fmt.Printf("🔎 [TRIAGE] Analisando '%s' com o modelo '%s'...\n", baseName, triageModel)
	}

	var triageRes *TriageResult
	triageErr := retryWithBackoff(2, func() error {
		var e error
		triageRes, e = client.TriageImage(imagePath, prompts.GetTriagePrompt(), triageModel)
		return e
	})
	if triageErr != nil {
		// Fail-open: em caso de erro (rate limit, API fora, parse), colore mesmo assim
		if opts.Verbose {
			fmt.Printf("⚠️ [TRIAGE] Falha na triagem de '%s': %v (fail-open: colorindo mesmo assim)\n", baseName, triageErr)
		}
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
