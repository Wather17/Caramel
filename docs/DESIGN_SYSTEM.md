# 🎨 Guia de Estilo e Sistema de Cores (Design System)

Este documento define a identidade visual, paleta de cores e diretrizes de interface em linha de comando (TUI/CLI) para o **Caramel CLI**.

---

## 🎵 Inspiração e Conceito

O nome e a identidade do **Caramel** são inspirados na faixa *"Caramel"* da banda *Even in Arcadia*. A estética busca transmitir elegância, calor e modernidade técnica através de tons magenta-amora combinados com acentos caramelo e cinza neutro.

---

## 🎨 Paleta de Cores Oficial

A interface TUI do Caramel utiliza uma paleta enxuta de **4 cores principais** para garantir consistência visual, hierarquia clara e excelente legibilidade em terminais claros e escuros.

| Papel Visual | HEX | Exemplo de Aplicação em Terminal |
| :--- | :--- | :--- |
| **Primary (Brand)** | `#C02B61` | Títulos principais, logotipos, bordas ativas e banners. |
| **Highlight (Ativo)** | `#E8709C` | Elemento focado/selecionado em listas (`> [X]`), textos ativos. |
| **Tag Accent (Acento)** | `#D96B27` | Badges de formato/extensão (`[DOCX]`, `[PDF]`), contadores e alertas suaves. |
| **Muted (Secundário)** | `#A19BA8` | Itens desmarcados (`[ ]`), dicas de teclado (`esc to cancel`), caminhos secundários. |

---

## 🛠️ Aplicação no Código Go (Lipgloss / Charm TUI)

Para implementar os estilos no código Go utilizando `github.com/charmbracelet/lipgloss`, siga as constantes padronizadas:

```go
package ui

import "github.com/charmbracelet/lipgloss"

// Cores Oficiais do Caramel CLI
var (
	ColorPrimary   = lipgloss.Color("#C02B61")
	ColorHighlight = lipgloss.Color("#E8709C")
	ColorTag       = lipgloss.Color("#D96B27")
	ColorMuted     = lipgloss.Color("#A19BA8")
)

// Estilos Reutilizáveis
var (
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary)

	SelectedItemStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorHighlight)

	UnselectedItemStyle = lipgloss.NewStyle().
				Foreground(ColorMuted)

	TagStyle = lipgloss.NewStyle().
			Foreground(ColorTag).
			Bold(true)

	HintStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Italic(true)
)
```

---

## 📐 Regras de Uso e Boas Práticas

1. **Economia de Cores**: Não misture cores fora da paleta sem necessidade. O excesso de cores gera ruído visual no terminal.
2. **Contraste e Legibilidade**: Certifique-se de que `#A19BA8` (Muted) permaneça legível tanto em fundos pretos/escuros quanto em fundos transparentes de terminal.
3. **Hierarquia Clara**:
   - **`#C02B61`** sempre indica **onde o usuário está** (cabeçalhos, nome do comando).
   - **`#E8709C`** sempre indica **o que o usuário está escolhendo agora**.
   - **`#D96B27`** destaca **metadados e tipos de arquivos**.
   - **`#A19BA8`** é usado para informações passivas ou auxiliares.
