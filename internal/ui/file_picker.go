package ui

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/text/unicode/norm"
)

// FilePickerItem representa um arquivo que pode ser escolhido em uma TUI.
// Name é o texto principal e Description contextualiza a origem do arquivo.
type FilePickerItem struct {
	Path        string
	Name        string
	Description string
}

// FilePickerAction descreve como o seletor foi encerrado.
type FilePickerAction string

const (
	FilePickerConfirmed FilePickerAction = "confirmed"
	FilePickerReveal    FilePickerAction = "reveal"
	FilePickerCanceled  FilePickerAction = "canceled"
)

// FilePickerResult contém a ação e os caminhos escolhidos pelo usuário.
type FilePickerResult struct {
	Action     FilePickerAction
	Selected   []string
	RevealPath string
}

// FilePickerModel é o estado testável do seletor de arquivos.
type FilePickerModel struct {
	title       string
	items       []FilePickerItem
	minimum     int
	query       textinput.Model
	cursor      int
	selected    map[string]bool
	filtered    []int
	status      string
	action      FilePickerAction
	revealPath  string
	windowWidth int
}

// NewFilePickerModel prepara um seletor com busca incremental e seleção múltipla.
func NewFilePickerModel(title string, items []FilePickerItem, minimum int) FilePickerModel {
	query := textinput.New()
	query.Prompt = "Buscar: "
	query.Placeholder = "nome ou caminho"
	query.CharLimit = 240
	query.Width = 64
	query.Focus()
	m := FilePickerModel{
		title:       title,
		items:       append([]FilePickerItem(nil), items...),
		minimum:     minimum,
		query:       query,
		selected:    make(map[string]bool),
		action:      FilePickerCanceled,
		windowWidth: 100,
	}
	m.refreshFilter()
	return m
}

// RunFilePicker abre o seletor em tela alternativa e retorna a ação escolhida.
func RunFilePicker(title string, items []FilePickerItem, minimum int) (FilePickerResult, error) {
	if len(items) == 0 {
		return FilePickerResult{}, fmt.Errorf("nenhum arquivo disponível para seleção")
	}
	model := NewFilePickerModel(title, items, minimum)
	finalModel, err := tea.NewProgram(model, tea.WithAltScreen()).Run()
	if err != nil {
		return FilePickerResult{}, fmt.Errorf("falha ao executar seletor de arquivos: %w", err)
	}
	result, ok := finalModel.(FilePickerModel)
	if !ok {
		return FilePickerResult{}, fmt.Errorf("seletor de arquivos retornou estado inválido")
	}
	return result.Result(), nil
}

// Result converte o estado final em um resultado estável para o chamador.
func (m FilePickerModel) Result() FilePickerResult {
	result := FilePickerResult{Action: m.action, RevealPath: m.revealPath}
	if m.action != FilePickerConfirmed {
		return result
	}
	for _, item := range m.items {
		if m.selected[item.Path] {
			result.Selected = append(result.Selected, item.Path)
		}
	}
	return result
}

func (m FilePickerModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m FilePickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case tea.WindowSizeMsg:
		m.windowWidth = typed.Width
		return m, nil
	case tea.KeyMsg:
		switch typed.Type {
		case tea.KeyCtrlC, tea.KeyEscape:
			m.action = FilePickerCanceled
			return m, tea.Quit
		case tea.KeyCtrlO:
			if index, ok := m.currentItem(); ok {
				m.revealPath = m.items[index].Path
				m.action = FilePickerReveal
				return m, tea.Quit
			}
			m.status = "Nenhum arquivo visível para revelar."
			return m, nil
		case tea.KeyEnter:
			if count := m.selectedCount(); count < m.minimum {
				m.status = fmt.Sprintf("Selecione pelo menos %d arquivo(s) (%d selecionado(s)).", m.minimum, count)
				return m, nil
			}
			m.action = FilePickerConfirmed
			return m, tea.Quit
		case tea.KeyUp:
			m.moveCursor(-1)
			return m, nil
		case tea.KeyDown:
			m.moveCursor(1)
			return m, nil
		case tea.KeySpace:
			m.toggleCurrent()
			return m, nil
		case tea.KeyRunes:
			if len(typed.Runes) == 1 && typed.Runes[0] == ' ' {
				m.toggleCurrent()
				return m, nil
			}
		}
	}

	updated, cmd := m.query.Update(msg)
	m.query = updated
	m.refreshFilter()
	return m, cmd
}

func (m FilePickerModel) View() string {
	var b strings.Builder
	b.WriteString(TitleStyle.Render(m.title))
	b.WriteString("\n\n")
	b.WriteString(m.query.View())
	b.WriteString("\n\n")
	if len(m.filtered) == 0 {
		b.WriteString(HintStyle.Render("Nenhum arquivo corresponde à busca."))
	} else {
		limit := len(m.filtered)
		if limit > 12 {
			limit = 12
		}
		for position, index := range m.filtered[:limit] {
			item := m.items[index]
			marker := "[ ]"
			if m.selected[item.Path] {
				marker = "[x]"
			}
			line := fmt.Sprintf("%s %s", marker, item.Name)
			if item.Description != "" {
				line += "  " + HintStyle.Render(item.Description)
			}
			if position == m.cursor {
				line = SelectedItemStyle.Render("> " + line)
			} else {
				line = UnselectedItemStyle.Render("  " + line)
			}
			b.WriteString(line)
			b.WriteByte('\n')
		}
		if len(m.filtered) > limit {
			b.WriteString(HintStyle.Render(fmt.Sprintf("… e mais %d arquivo(s)", len(m.filtered)-limit)))
			b.WriteByte('\n')
		}
	}
	if m.status != "" {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(ColorWarning).Render(m.status))
	}
	b.WriteString("\n\n")
	b.WriteString(HintStyle.Render("↑/↓ navegar · Espaço selecionar · Enter confirmar · Ctrl+O revelar · Esc cancelar"))
	return b.String()
}

func (m *FilePickerModel) refreshFilter() {
	query := normalizePickerText(m.query.Value())
	m.filtered = m.filtered[:0]
	for index, item := range m.items {
		if query == "" || strings.Contains(normalizePickerText(item.Name), query) || strings.Contains(normalizePickerText(item.Path), query) || strings.Contains(normalizePickerText(item.Description), query) {
			m.filtered = append(m.filtered, index)
		}
	}
	if len(m.filtered) == 0 {
		m.cursor = 0
		return
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
}

func (m FilePickerModel) currentItem() (int, bool) {
	if m.cursor < 0 || m.cursor >= len(m.filtered) {
		return 0, false
	}
	return m.filtered[m.cursor], true
}

func (m *FilePickerModel) moveCursor(delta int) {
	if len(m.filtered) == 0 {
		return
	}
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = len(m.filtered) - 1
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = 0
	}
}

func (m *FilePickerModel) toggleCurrent() {
	index, ok := m.currentItem()
	if !ok {
		return
	}
	if m.selected == nil {
		m.selected = make(map[string]bool)
	}
	path := m.items[index].Path
	m.selected[path] = !m.selected[path]
}

func (m FilePickerModel) selectedCount() int {
	count := 0
	for _, item := range m.items {
		if m.selected[item.Path] {
			count++
		}
	}
	return count
}

func normalizePickerText(value string) string {
	value = strings.ToLower(value)
	value = norm.NFD.String(value)
	return strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Mn, r) {
			return -1
		}
		return r
	}, value)
}
