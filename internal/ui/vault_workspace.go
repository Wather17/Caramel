package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"caramel/internal/config"
	"caramel/internal/tools/ai"
	"caramel/internal/vault"
	"caramel/internal/workflow"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type vaultScreen int

const (
	vaultInbox vaultScreen = iota
	vaultImport
	vaultGenerate
	vaultConfirm
	vaultRunning
	vaultConsole
)

type vaultRunMessage struct {
	event     workflow.ProgressEvent
	done      bool
	materials []vault.Material
	err       error
	channel   <-chan vaultRunMessage
}

// VaultWorkspaceModel é a experiência de inbox e busca do acervo global.
type VaultWorkspaceModel struct {
	vault                *vault.Vault
	screen               vaultScreen
	materials            []vault.Material
	cursor               int
	selected             map[string]bool
	query                textinput.Model
	input                textinput.Model
	inputPrompt          string
	status               string
	fatal                error
	pendingOp            string
	pendingText          []string
	collection           string
	runCancel            context.CancelFunc
	runEvents            []workflow.ProgressEvent
	consoleInput         textinput.Model
	consoleLines         []string
	consoleHistory       []string
	consoleHistoryCursor int
	returnToConsole      bool
}

// NewVaultWorkspaceModel abre o vault, migra projetos legados e prepara a inbox.
func NewVaultWorkspaceModel() VaultWorkspaceModel {
	query := textinput.New()
	query.CharLimit = 180
	query.Width = 58
	input := textinput.New()
	input.CharLimit = 240
	input.Width = 72
	consoleInput := textinput.New()
	consoleInput.CharLimit = 500
	consoleInput.Width = 78
	consoleInput.Prompt = "> "
	m := VaultWorkspaceModel{screen: vaultInbox, query: query, input: input, consoleInput: consoleInput, selected: map[string]bool{}}
	v, err := vault.Open()
	if err != nil {
		m.fatal = err
		return m
	}
	m.vault = v
	report, err := v.MigrateLegacyProjects(context.Background())
	if err != nil {
		m.status = fmt.Sprintf("⚠️ Migração parcial: %v", err)
	} else if report.Projects > 0 {
		m.status = fmt.Sprintf("✅ %d projeto(s) legado(s) incorporado(s) ao vault", report.Projects)
	}
	m.reload()
	return m
}

// RunVaultWorkspace executa a TUI e fecha o banco ao sair.
func RunVaultWorkspace() error {
	model := NewVaultWorkspaceModel()
	if model.fatal != nil {
		return model.fatal
	}
	_, err := tea.NewProgram(model, tea.WithAltScreen()).Run()
	closeErr := model.vault.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func (m VaultWorkspaceModel) Init() tea.Cmd { return textinput.Blink }

func (m VaultWorkspaceModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case tea.WindowSizeMsg:
		return m, nil
	case vaultRunMessage:
		if typed.done {
			m.runCancel = nil
			m.reload()
			if m.returnToConsole {
				m.appendConsoleOutput(formatWorkflowCompletion(len(typed.materials), typed.err))
				m.returnToConsole = false
				m.screen = vaultConsole
				m.consoleInput.Focus()
			} else {
				m.status = formatWorkflowCompletion(len(typed.materials), typed.err)
				m.screen = vaultInbox
			}
			return m, nil
		}
		m.runEvents = append(m.runEvents, typed.event)
		if m.returnToConsole && typed.event.Message != "" && typed.event.Step != "done" {
			m.appendConsoleOutput(formatWorkflowEvent(typed.event))
		}
		if len(m.runEvents) > 10 {
			m.runEvents = m.runEvents[len(m.runEvents)-10:]
		}
		return m, waitVaultRun(typed.channel)
	case tea.KeyMsg:
		if typed.String() == "ctrl+c" {
			if m.screen == vaultRunning && m.runCancel != nil {
				m.runCancel()
				m.status = "⏳ Cancelamento solicitado..."
				return m, nil
			}
			return m, tea.Quit
		}
	}

	switch m.screen {
	case vaultInbox:
		return m.updateInbox(msg)
	case vaultImport, vaultGenerate:
		return m.updateInput(msg)
	case vaultConfirm:
		return m.updateConfirm(msg)
	case vaultRunning:
		if key, ok := msg.(tea.KeyMsg); ok && (key.String() == "esc" || key.String() == "c") && m.runCancel != nil {
			m.runCancel()
			m.status = "⏳ Cancelamento solicitado..."
		}
	case vaultConsole:
		return m.updateConsole(msg)
	}
	return m, nil
}

func (m VaultWorkspaceModel) updateInbox(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.query.Focused() {
		if key, ok := msg.(tea.KeyMsg); ok {
			switch key.String() {
			case "enter":
				m.query.Blur()
				m.reload()
				return m, nil
			case "esc":
				m.query.Blur()
				return m, nil
			}
		}
		var cmd tea.Cmd
		m.query, cmd = m.query.Update(msg)
		return m, cmd
	}
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "/":
		m.query.Focus()
	case ":":
		m.openConsole()
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.materials)-1 {
			m.cursor++
		}
	case "space":
		if len(m.materials) > 0 {
			id := m.materials[m.cursor].ID
			m.selected[id] = !m.selected[id]
		}
	case "a":
		for _, material := range m.materials {
			m.selected[material.ID] = true
		}
	case "n":
		for _, material := range m.materials {
			m.selected[material.ID] = false
		}
	case "i":
		m.beginInput(vaultImport, "Arquivo ou pasta de imagens", "./imagens")
	case "g":
		m.beginInput(vaultGenerate, "Itens separados por vírgula", "maçã, banana, uva")
	case "c":
		m.prepareRun("colorize", nil)
	case "f":
		m.prepareRun("cards", nil)
	case "p":
		m.prepareRun("2up", nil)
	case "r":
		m.reload()
	case "q":
		return m, tea.Quit
	}
	return m, nil
}

func (m VaultWorkspaceModel) updateInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			m.input.Blur()
			m.screen = vaultInbox
			return m, nil
		case "enter":
			value := strings.TrimSpace(m.input.Value())
			if value == "" {
				m.status = "⚠️ Informe um valor antes de continuar."
				return m, nil
			}
			if m.screen == vaultImport {
				results, err := m.vault.ImportPaths(context.Background(), []string{value}, []string{"importado"})
				if err != nil {
					m.status = fmt.Sprintf("❌ %v", err)
					return m, nil
				}
				for _, result := range results {
					m.selected[result.Material.ID] = true
				}
				m.input.Blur()
				m.screen = vaultInbox
				m.reload()
				m.status = fmt.Sprintf("✅ %d material(is) importado(s)", len(results))
				return m, nil
			}
			m.input.Blur()
			m.prepareRun("generate", splitItems(value))
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m VaultWorkspaceModel) updateConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "enter", "y":
		return m.startRun(m.pendingOp, m.pendingText)
	case "esc", "n":
		m.screen = vaultInbox
		m.pendingOp = ""
		m.pendingText = nil
		m.status = "Operação não iniciada."
	}
	return m, nil
}

func (m VaultWorkspaceModel) prepareRun(operation string, text []string) {
	if operation != "generate" && len(m.selectedIDs()) == 0 {
		m.status = "⚠️ Selecione pelo menos um material. Use espaço ou a."
		return
	}
	if operation == "generate" && len(text) == 0 {
		m.status = "⚠️ Informe pelo menos um item."
		return
	}
	m.pendingOp = operation
	m.pendingText = append([]string(nil), text...)
	m.screen = vaultConfirm
	m.status = ""
}

func (m VaultWorkspaceModel) startRun(operation string, text []string) (tea.Model, tea.Cmd) {
	collectionID, err := m.ensureCollection()
	if err != nil {
		m.status = fmt.Sprintf("❌ %v", err)
		return m, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan vaultRunMessage, 32)
	ids := m.selectedIDs()
	m.collection = collectionID
	m.runCancel = cancel
	m.runEvents = nil
	m.screen = vaultRunning
	go func() {
		service := workflow.VaultImageService{Vault: m.vault, CollectionID: collectionID, Progress: func(event workflow.ProgressEvent) {
			ch <- vaultRunMessage{event: event}
		}}
		var (
			materials []vault.Material
			runErr    error
		)
		switch operation {
		case "generate":
			materials, runErr = service.Generate(ctx, workflow.ImageOptions{Items: text})
		case "colorize":
			materials, runErr = service.Colorize(ctx, ids, workflow.ImageOptions{})
		case "cards":
			materials, runErr = service.Cards(ctx, ids)
		case "2up":
			materials, runErr = service.TwoUp(ctx, ids)
		default:
			runErr = fmt.Errorf("operação desconhecida: %s", operation)
		}
		ch <- vaultRunMessage{done: true, materials: materials, err: runErr}
		close(ch)
	}()
	return m, waitVaultRun(ch)
}

func waitVaultRun(ch <-chan vaultRunMessage) tea.Cmd {
	return func() tea.Msg {
		message, ok := <-ch
		if !ok {
			return vaultRunMessage{done: true}
		}
		message.channel = ch
		return message
	}
}

func (m VaultWorkspaceModel) ensureCollection() (string, error) {
	if m.collection != "" {
		return m.collection, nil
	}
	collection, err := m.vault.CreateCollection(context.Background(), "Sessão "+time.Now().Format("02/01/2006 15:04"), true, "")
	if err != nil {
		return "", err
	}
	return collection.ID, nil
}

func (m *VaultWorkspaceModel) reload() {
	if m.vault == nil {
		return
	}
	materials, err := m.vault.Search(context.Background(), vault.SearchOptions{Query: m.query.Value()})
	if err != nil {
		m.fatal = err
		return
	}
	m.materials = materials
	if m.cursor >= len(materials) {
		m.cursor = len(materials) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *VaultWorkspaceModel) beginInput(screen vaultScreen, prompt, placeholder string) {
	m.screen = screen
	m.inputPrompt = prompt
	m.input.SetValue("")
	m.input.Placeholder = placeholder
	m.input.Focus()
	m.status = ""
}

func (m VaultWorkspaceModel) selectedIDs() []string {
	var ids []string
	for _, material := range m.materials {
		if m.selected[material.ID] {
			ids = append(ids, material.ID)
		}
	}
	return ids
}

func (m VaultWorkspaceModel) View() string {
	if m.fatal != nil {
		return fmt.Sprintf("%s\n\n❌ %v\n", TitleStyle.Render("🍬 Caramel Vault"), m.fatal)
	}
	var b strings.Builder
	b.WriteString(TitleStyle.Render("🍬 Caramel Vault"))
	b.WriteString("\n")
	switch m.screen {
	case vaultInbox:
		b.WriteString(HelpSectionTitleStyle.Render("Inbox de materiais"))
		b.WriteString("\n")
		b.WriteString(HintStyle.Render("Busca: "))
		b.WriteString(m.query.View())
		b.WriteString("\n\n")
		if len(m.materials) == 0 {
			b.WriteString("Nenhum material encontrado. Pressione i para importar imagens.\n")
		} else {
			for i, material := range m.materials {
				prefix, mark := "  ", "[ ]"
				style := UnselectedItemStyle
				if m.selected[material.ID] {
					mark = "[x]"
				}
				if i == m.cursor {
					prefix = "› "
					style = SelectedItemStyle
				}
				b.WriteString(style.Render(fmt.Sprintf("%s%s %-30s", prefix, mark, material.Title)))
				kind := material.Kind
				if material.Category != "" {
					kind += " · " + material.Category
				}
				b.WriteString(HintStyle.Render(fmt.Sprintf("  %s  %s", kind, strings.Join(material.Tags, ", "))))
				b.WriteString("\n")
			}
			if len(m.materials) > 0 {
				material := m.materials[m.cursor]
				if material.Kind == "image" {
					if preview, err := RenderImageFileToANSI(m.vault.ObjectPath(material), 34, 8); err == nil {
						b.WriteString("\n" + preview)
					}
				}
			}
		}
		b.WriteString("\n")
		b.WriteString(HintStyle.Render("↑/↓ navegar · espaço selecionar · a todos · n nenhum · / buscar · : console · i importar · g gerar · c colorir · f fichas · p 2-up · q sair"))
	case vaultImport, vaultGenerate:
		b.WriteString(HelpSectionTitleStyle.Render(m.inputPrompt))
		b.WriteString("\n\n")
		b.WriteString(m.input.View())
		b.WriteString("\n\n")
		b.WriteString(HintStyle.Render("enter confirmar · esc voltar"))
	case vaultConfirm:
		b.WriteString(HelpSectionTitleStyle.Render("Confirmar operação"))
		b.WriteString("\n\n")
		b.WriteString(fmt.Sprintf("Operação: %s\n", operationLabel(m.pendingOp)))
		if m.pendingOp == "generate" {
			b.WriteString(fmt.Sprintf("Itens: %s\n", strings.Join(m.pendingText, ", ")))
			b.WriteString(fmt.Sprintf("Modelo de imagem: %s\n", configuredModel(ai.DefaultModel, func(cfg config.Config) string { return cfg.ModelImage })))
			b.WriteString(fmt.Sprintf("Modelo de texto: %s\n", configuredModel(ai.DefaultTextModel, func(cfg config.Config) string { return cfg.ModelText })))
		} else {
			b.WriteString(fmt.Sprintf("Materiais selecionados: %d\n", len(m.selectedIDs())))
			if m.pendingOp == "colorize" {
				b.WriteString(fmt.Sprintf("Modelo de imagem: %s\n", configuredModel(ai.DefaultModel, func(cfg config.Config) string { return cfg.ModelImage })))
			}
		}
		b.WriteString("Destino: vault global\n\n")
		b.WriteString(HintStyle.Render("enter/y confirmar · esc/n cancelar"))
	case vaultRunning:
		b.WriteString(HelpSectionTitleStyle.Render("Executando no vault"))
		b.WriteString("\n\n")
		for _, event := range m.runEvents {
			b.WriteString(formatWorkflowEvent(event))
			b.WriteString("\n")
		}
		if len(m.runEvents) == 0 {
			b.WriteString("Preparando operação...\n")
		}
		b.WriteString("\n" + HintStyle.Render("esc/c cancelar com segurança"))
	case vaultConsole:
		b.WriteString(HelpSectionTitleStyle.Render("Console do Caramel"))
		b.WriteString("\n\n")
		start := 0
		if len(m.consoleLines) > 18 {
			start = len(m.consoleLines) - 18
		}
		for _, line := range m.consoleLines[start:] {
			b.WriteString(line)
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString(m.consoleInput.View())
		b.WriteString("\n\n" + HintStyle.Render("enter executar · ↑/↓ histórico · esc inbox"))
	}
	if m.status != "" {
		b.WriteString("\n\n" + lipgloss.NewStyle().Foreground(ColorWarning).Render(m.status))
	}
	return b.String()
}
