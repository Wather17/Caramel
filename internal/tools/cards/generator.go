package cards

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
)

// CardItem representa um cartão/ficha individual na folha
type CardItem struct {
	Name        string
	ImagePath   string
	DataURI     string
	Index       int
}

// SheetOptions configura a diagramação da folha A4
type SheetOptions struct {
	Columns     int    // 2 (padrão), 3 ou 4
	Rows        int    // 3 (padrão), 2 ou 4
	Title       string // Título no cabeçalho da folha
	CutLines    bool   // Exibe linhas tracejadas de corte
	Uppercase   bool   // Caixa alta no nome (ex: MAÇÃ) para alfabetização
	EmbedBase64 bool   // Embute imagens como Data URI (arquivo 100% independente)
}

// DefaultOptions retorna a configuração padrão (6 fichas por folha A4)
func DefaultOptions() SheetOptions {
	return SheetOptions{
		Columns:     2,
		Rows:        3,
		CutLines:    true,
		Uppercase:   true,
		EmbedBase64: true,
	}
}

// PageLayout agrupa as fichas distribuídas por folha A4
type PageLayout struct {
	PageIndex  int
	TotalPages int
	Title      string
	Cards      []CardItem
}

// HTMLTemplateData dados passados para renderizar o HTML
type HTMLTemplateData struct {
	Title       string
	Pages       []PageLayout
	Columns     int
	Rows        int
	CutLines    bool
	Uppercase   bool
	CardsPerPage int
}

const htmlTemplate = `<!DOCTYPE html>
<html lang="pt-BR">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{{ if .Title }}{{ .Title }}{{ else }}Fichas Pedagógicas - Caramel{{ end }}</title>
  <script src="https://cdn.tailwindcss.com"></script>
  <style>
    @import url('https://fonts.googleapis.com/css2?family=Outfit:wght@400;600;700;800&family=Nunito:wght@700;800;900&display=swap');
    
    body {
      font-family: 'Outfit', sans-serif;
      background-color: #f1f5f9;
      margin: 0;
      padding: 20px 0;
      color: #1e293b;
    }

    @page {
      size: A4 portrait;
      margin: 8mm;
    }

    .a4-sheet {
      width: 210mm;
      min-height: 297mm;
      height: 297mm;
      margin: 0 auto 20px auto;
      background: white;
      padding: 8mm;
      box-sizing: border-box;
      box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -1px rgba(0, 0, 0, 0.06);
      display: flex;
      flex-direction: column;
      page-break-after: always;
      break-after: page;
      position: relative;
    }

    @media print {
      body {
        background: transparent;
        padding: 0;
      }
      .no-print {
        display: none !important;
      }
      .a4-sheet {
        margin: 0;
        box-shadow: none;
        width: 100%;
        height: 100%;
        min-height: auto;
        padding: 0;
      }
    }

    .card-grid {
      display: grid;
      grid-template-columns: repeat({{ .Columns }}, minmax(0, 1fr));
      grid-template-rows: repeat({{ .Rows }}, minmax(0, 1fr));
      gap: 12px;
      flex-grow: 1;
      height: 100%;
    }

    .card-item {
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: space-between;
      padding: 10px;
      box-sizing: border-box;
      border-radius: 8px;
      position: relative;
      background: #ffffff;
      height: 100%;
      min-height: 0;
      break-inside: avoid;
    }

    .card-border-dashed {
      border: 2px dashed #cbd5e1;
    }

    .card-image-box {
      width: 100%;
      flex-grow: 1;
      min-height: 0;
      display: flex;
      align-items: center;
      justify-content: center;
    }

    .card-image-box img {
      max-width: 100%;
      max-height: 100%;
      object-fit: contain;
      aspect-ratio: 1 / 1;
    }

    .card-label {
      font-family: 'Nunito', sans-serif;
      font-weight: 800;
      letter-spacing: 0.05em;
    }
  </style>
</head>
<body>

  <!-- Barra de Controle Superior (Não aparece na impressão) -->
  <div class="no-print max-w-[210mm] mx-auto mb-6 bg-white p-4 rounded-xl shadow-md flex items-center justify-between border border-slate-200">
    <div>
      <h1 class="text-xl font-bold text-slate-800">📄 Layout de Fichas Pedagógicas (A4)</h1>
      <p class="text-sm text-slate-500">Pronto para impressão direta ou corte em tesoura.</p>
    </div>
    <div class="flex items-center gap-3">
      <button onclick="window.print()" class="px-5 py-2.5 bg-amber-500 hover:bg-amber-600 active:scale-95 text-white font-bold rounded-lg shadow transition flex items-center gap-2">
        <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 17h2a2 2 0 002-2v-4a2 2 0 00-2-2H5a2 2 0 00-2 2v4a2 2 0 002 2h2m2 4h6a2 2 0 002-2v-4a2 2 0 00-2-2H9a2 2 0 00-2 2v4a2 2 0 002 2zm8-12V5a2 2 0 00-2-2H9a2 2 0 00-2 2v4h10z"></path></svg>
        Imprimir / Salvar PDF
      </button>
    </div>
  </div>

  {{ range $page := .Pages }}
  <div class="a4-sheet">
    {{ if $.Title }}
    <header class="mb-3 pb-2 border-b border-slate-200 flex justify-between items-end">
      <h2 class="text-lg font-bold text-slate-700 uppercase tracking-wide">{{ $.Title }}</h2>
      <span class="text-xs font-semibold text-slate-400">Pág. {{ $page.PageIndex }} / {{ $page.TotalPages }}</span>
    </header>
    {{ end }}

    <div class="card-grid">
      {{ range $card := $page.Cards }}
      <div class="card-item {{ if $.CutLines }}card-border-dashed{{ else }}border border-slate-200{{ end }}">
        {{ if $.CutLines }}
        <span class="absolute -top-2.5 left-4 bg-white px-1 text-[11px] text-slate-400 no-print select-none">✂️</span>
        {{ end }}
        
        <div class="card-image-box">
          <img src="{{ if $card.DataURI }}{{ $card.DataURI | safeURL }}{{ else }}{{ $card.ImagePath }}{{ end }}" alt="{{ $card.Name }}" loading="lazy" />
        </div>

        <div class="w-full text-center mt-2 pt-2 border-t border-slate-100">
          <span class="card-label text-xl text-slate-900 block leading-tight">
            {{ if $.Uppercase }}{{ $card.Name | printf "%s" | upper }}{{ else }}{{ $card.Name }}{{ end }}
          </span>
        </div>
      </div>
      {{ end }}
    </div>
  </div>
  {{ end }}

</body>
</html>
`

// GenerateCardsHTML gera o arquivo HTML com as folhas A4 diagramadas
func GenerateCardsHTML(items []CardItem, outputPath string, opts SheetOptions) error {
	if len(items) == 0 {
		return fmt.Errorf("nenhum item fornecido para gerar fichas A4")
	}

	cols := opts.Columns
	if cols <= 0 {
		cols = 2
	}
	rows := opts.Rows
	if rows <= 0 {
		rows = 3
	}
	cardsPerPage := cols * rows

	// Processa DataURIs se solicitado para arquivo 100% autônomo
	processedItems := make([]CardItem, len(items))
	for i, item := range items {
		processedItems[i] = item
		processedItems[i].Index = i + 1

		if opts.EmbedBase64 && item.ImagePath != "" {
			data, err := os.ReadFile(item.ImagePath)
			if err == nil {
				ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(item.ImagePath), "."))
				if ext == "jpg" {
					ext = "jpeg"
				}
				if ext == "" {
					ext = "png"
				}
				b64 := base64.StdEncoding.EncodeToString(data)
				processedItems[i].DataURI = fmt.Sprintf("data:image/%s;base64,%s", ext, b64)
			}
		}
	}

	// Agrupa itens em páginas
	var pages []PageLayout
	totalPages := (len(processedItems) + cardsPerPage - 1) / cardsPerPage

	for pageIdx := 0; pageIdx < totalPages; pageIdx++ {
		start := pageIdx * cardsPerPage
		end := start + cardsPerPage
		if end > len(processedItems) {
			end = len(processedItems)
		}

		pages = append(pages, PageLayout{
			PageIndex:  pageIdx + 1,
			TotalPages: totalPages,
			Title:      opts.Title,
			Cards:      processedItems[start:end],
		})
	}

	funcMap := template.FuncMap{
		"upper":   strings.ToUpper,
		"safeURL": func(s string) template.URL { return template.URL(s) },
	}

	tmpl, err := template.New("cards_sheet").Funcs(funcMap).Parse(htmlTemplate)
	if err != nil {
		return fmt.Errorf("falha ao compilar template HTML de fichas: %w", err)
	}

	data := HTMLTemplateData{
		Title:        opts.Title,
		Pages:        pages,
		Columns:      cols,
		Rows:         rows,
		CutLines:     opts.CutLines,
		Uppercase:    opts.Uppercase,
		CardsPerPage: cardsPerPage,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("falha ao executar template HTML: %w", err)
	}

	if err := os.WriteFile(outputPath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("falha ao salvar arquivo HTML em '%s': %w", outputPath, err)
	}

	return nil
}
