package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
)

type workspaceConsoleCommand struct {
	Name        string
	Usage       string
	Description string
	Aliases     []string
}

var workspaceConsoleCommands = []workspaceConsoleCommand{
	{Name: "help", Usage: "help", Description: "lista os comandos da workspace"},
	{Name: "search", Usage: "search [termo]", Description: "busca materiais no vault", Aliases: []string{"find"}},
	{Name: "import", Usage: "import <arquivo ou pasta>", Description: "importa imagens para o vault", Aliases: []string{"i"}},
	{Name: "generate", Usage: "generate <itens>", Description: "gera imagens a partir dos itens", Aliases: []string{"g"}},
	{Name: "colorize", Usage: "colorize", Description: "colore os materiais selecionados", Aliases: []string{"c"}},
	{Name: "cards", Usage: "cards", Description: "gera fichas dos materiais selecionados", Aliases: []string{"f"}},
	{Name: "2up", Usage: "2up", Description: "monta uma folha 2-up dos selecionados", Aliases: []string{"p"}},
	{Name: "clear", Usage: "clear", Description: "limpa a saída do console"},
	{Name: "back", Usage: "back", Description: "volta para a inbox"},
	{Name: "quit", Usage: "quit", Description: "fecha a workspace", Aliases: []string{"exit"}},
}

func parseConsoleInput(input string) ([]string, error) {
	var args []string
	var current strings.Builder
	var quote rune
	escaped := false
	tokenStarted := false
	flush := func() {
		if tokenStarted {
			args = append(args, current.String())
			current.Reset()
			tokenStarted = false
		}
	}

	for _, char := range input {
		if escaped {
			current.WriteRune(char)
			escaped = false
			tokenStarted = true
			continue
		}
		if char == '\\' {
			escaped = true
			tokenStarted = true
			continue
		}
		if quote != 0 {
			if char == quote {
				quote = 0
				continue
			}
			current.WriteRune(char)
			tokenStarted = true
			continue
		}
		if char == '\'' || char == '"' {
			quote = char
			tokenStarted = true
			continue
		}
		if unicode.IsSpace(char) {
			flush()
			continue
		}
		current.WriteRune(char)
		tokenStarted = true
	}
	if escaped {
		return nil, fmt.Errorf("barra de escape incompleta")
	}
	if quote != 0 {
		return nil, fmt.Errorf("aspas não fechadas")
	}
	flush()
	return args, nil
}

func resolveWorkspaceConsoleCommand(name string) (workspaceConsoleCommand, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, command := range workspaceConsoleCommands {
		if command.Name == name {
			return command, true
		}
		for _, alias := range command.Aliases {
			if alias == name {
				return command, true
			}
		}
	}
	return workspaceConsoleCommand{}, false
}

func workspaceConsoleHelpLines() []string {
	lines := []string{"Comandos disponíveis:"}
	for _, command := range workspaceConsoleCommands {
		lines = append(lines, fmt.Sprintf("  %-24s %s", command.Usage, command.Description))
	}
	lines = append(lines, "Aliases: find/search, i/import, g/generate, c/colorize, f/cards, p/2up")
	return lines
}

func expandConsolePath(path string) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~/") && !strings.HasPrefix(path, "~\\") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("não foi possível expandir '~': %w", err)
	}
	if path == "~" {
		return home, nil
	}
	return filepath.Join(home, path[2:]), nil
}

func (m *VaultWorkspaceModel) openConsole() {
	m.screen = vaultConsole
	m.consoleInput.SetValue("")
	m.consoleInput.Focus()
	m.consoleHistoryCursor = len(m.consoleHistory)
	if len(m.consoleLines) == 0 {
		m.consoleLines = append(m.consoleLines, "Console do Caramel. Digite help para listar os comandos.")
	}
}

func (m *VaultWorkspaceModel) appendConsoleOutput(lines ...string) {
	m.consoleLines = append(m.consoleLines, lines...)
	if len(m.consoleLines) > 80 {
		m.consoleLines = m.consoleLines[len(m.consoleLines)-80:]
	}
}

func (m VaultWorkspaceModel) updateConsole(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if ok {
		switch key.String() {
		case "esc":
			m.consoleInput.Blur()
			m.screen = vaultInbox
			return m, nil
		case "enter":
			return m.executeConsoleCommand(m.consoleInput.Value())
		case "up":
			if m.consoleHistoryCursor > 0 {
				m.consoleHistoryCursor--
				m.consoleInput.SetValue(m.consoleHistory[m.consoleHistoryCursor])
			}
			return m, nil
		case "down":
			if m.consoleHistoryCursor < len(m.consoleHistory)-1 {
				m.consoleHistoryCursor++
				m.consoleInput.SetValue(m.consoleHistory[m.consoleHistoryCursor])
			} else {
				m.consoleHistoryCursor = len(m.consoleHistory)
				m.consoleInput.SetValue("")
			}
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.consoleInput, cmd = m.consoleInput.Update(msg)
	return m, cmd
}

func (m VaultWorkspaceModel) executeConsoleCommand(raw string) (tea.Model, tea.Cmd) {
	input := strings.TrimSpace(raw)
	m.consoleInput.SetValue("")
	if input == "" {
		return m, nil
	}
	m.consoleHistory = append(m.consoleHistory, input)
	m.consoleHistoryCursor = len(m.consoleHistory)
	m.appendConsoleOutput("$ " + input)

	args, err := parseConsoleInput(input)
	if err != nil {
		m.appendConsoleOutput("❌ " + err.Error())
		return m, nil
	}
	command, ok := resolveWorkspaceConsoleCommand(args[0])
	if !ok {
		m.appendConsoleOutput(fmt.Sprintf("❌ comando desconhecido: %s (use help)", args[0]))
		return m, nil
	}

	switch command.Name {
	case "help":
		if len(args) != 1 {
			m.appendConsoleOutput("❌ help não aceita argumentos")
			return m, nil
		}
		m.appendConsoleOutput(workspaceConsoleHelpLines()...)
	case "clear":
		m.consoleLines = nil
	case "back":
		m.consoleInput.Blur()
		m.screen = vaultInbox
	case "quit":
		return m, tea.Quit
	case "search":
		if len(args) > 1 {
			m.query.SetValue(strings.Join(args[1:], " "))
		} else {
			m.query.SetValue("")
		}
		m.reload()
		m.appendConsoleOutput(fmt.Sprintf("✅ %d material(is) encontrado(s)", len(m.materials)))
	case "import":
		if len(args) < 2 {
			m.appendConsoleOutput("❌ uso: import <arquivo ou pasta>")
			return m, nil
		}
		path, pathErr := expandConsolePath(strings.Join(args[1:], " "))
		if pathErr != nil {
			m.appendConsoleOutput("❌ " + pathErr.Error())
			return m, nil
		}
		results, importErr := m.vault.ImportPaths(context.Background(), []string{path}, []string{"importado"})
		if importErr != nil {
			m.appendConsoleOutput("❌ " + importErr.Error())
			return m, nil
		}
		for _, result := range results {
			m.selected[result.Material.ID] = true
		}
		m.reload()
		m.appendConsoleOutput(fmt.Sprintf("✅ %d material(is) importado(s)", len(results)))
	case "generate":
		if len(args) < 2 {
			m.appendConsoleOutput("❌ uso: generate <itens separados por vírgula>")
			return m, nil
		}
		items := splitItems(strings.Join(args[1:], " "))
		if len(items) == 0 {
			m.appendConsoleOutput("❌ informe pelo menos um item")
			return m, nil
		}
		m.returnToConsole = true
		m.appendConsoleOutput("⏳ iniciando geração...")
		return m.startRun("generate", items)
	case "colorize", "cards", "2up":
		if len(args) != 1 {
			m.appendConsoleOutput(fmt.Sprintf("❌ %s não aceita argumentos", command.Name))
			return m, nil
		}
		if len(m.selectedIDs()) == 0 {
			m.appendConsoleOutput("❌ selecione pelo menos um material na inbox")
			return m, nil
		}
		m.returnToConsole = true
		m.appendConsoleOutput(fmt.Sprintf("⏳ iniciando %s...", command.Name))
		return m.startRun(command.Name, nil)
	}
	return m, nil
}
