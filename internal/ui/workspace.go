package ui

import (
	"context"
	"fmt"
	"strings"

	"caramel/internal/config"
	"caramel/internal/tools/ai"
	"caramel/internal/workflow"
	"caramel/internal/workspace"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type workspaceScreen int

const (
	screenProjects workspaceScreen = iota
	screenNewProject
	screenDashboard
	screenInputPath
	screenGenerate
	screenSelectImages
	screenConfirm
	screenRunning
)

type workspaceRunMessage struct {
	event     workflow.ProgressEvent
	done      bool
	artifacts []workspace.Artifact
	err       error
	channel   <-chan workspaceRunMessage
}

// WorkspaceModel é o shell Bubble Tea da área de trabalho do Caramel.
type WorkspaceModel struct {
	screen       workspaceScreen
	projects     []*workspace.Project
	project      *workspace.Project
	cursor       int
	input        textinput.Model
	inputPrompt  string
	files        []workspace.ImageFile
	selected     map[string]bool
	status       string
	runEvents    []workflow.ProgressEvent
	runCancel    context.CancelFunc
	pendingOp    string
	pendingItems []string
	width        int
	height       int
	fatal        error
}

// NewWorkspaceModel cria a tela inicial e carrega os projetos existentes.
func NewWorkspaceModel() WorkspaceModel {
	input := textinput.New()
	input.CharLimit = 240
	input.Width = 72
	m := WorkspaceModel{screen: screenProjects, input: input, selected: map[string]bool{}}
	m.reloadProjects()
	return m
}

// RunWorkspace abre a TUI de projetos.
func RunWorkspace() error {
	return RunVaultWorkspace()
}

func (m WorkspaceModel) Init() tea.Cmd { return textinput.Blink }

func (m WorkspaceModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = typed.Width, typed.Height
		return m, nil
	case workspaceRunMessage:
		if typed.done {
			m.runCancel = nil
			m.status = formatWorkflowCompletion(len(typed.artifacts), typed.err)
			if m.project != nil {
				if refreshed, err := workspace.OpenProject(m.project.ID); err == nil {
					m.project = refreshed
					m.refreshFiles()
				}
			}
			m.screen = screenDashboard
			return m, nil
		}
		m.runEvents = append(m.runEvents, typed.event)
		if len(m.runEvents) > 8 {
			m.runEvents = m.runEvents[len(m.runEvents)-8:]
		}
		return m, waitWorkspaceRun(typed.channel)
	case tea.KeyMsg:
		if typed.String() == "ctrl+c" {
			if m.screen == screenRunning && m.runCancel != nil {
				m.runCancel()
				m.status = "⏳ Cancelamento solicitado; aguardando a etapa atual terminar..."
				return m, nil
			}
			return m, tea.Quit
		}
		if typed.String() == "q" && m.screen != screenNewProject && m.screen != screenInputPath && m.screen != screenGenerate && m.screen != screenSelectImages && m.screen != screenRunning {
			return m, tea.Quit
		}
	}

	switch m.screen {
	case screenProjects:
		return m.updateProjects(msg)
	case screenNewProject, screenInputPath, screenGenerate:
		return m.updateInputScreen(msg)
	case screenDashboard:
		return m.updateDashboard(msg)
	case screenSelectImages:
		return m.updateSelection(msg)
	case screenConfirm:
		return m.updateConfirm(msg)
	case screenRunning:
		if key, ok := msg.(tea.KeyMsg); ok && (key.String() == "esc" || key.String() == "c") {
			if m.runCancel != nil {
				m.runCancel()
				m.status = "⏳ Cancelamento solicitado; aguardando a etapa atual terminar..."
			}
		}
	}
	return m, nil
}

func (m WorkspaceModel) updateProjects(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.projects)-1 {
			m.cursor++
		}
	case "n":
		m.beginInput(screenNewProject, "Nome do novo projeto", "ex: Atividades de animais")
	case "enter":
		if len(m.projects) > 0 {
			m.project = m.projects[m.cursor]
			m.refreshFiles()
			m.screen = screenDashboard
			m.status = ""
		}
	case "r":
		m.reloadProjects()
	}
	return m, nil
}

func (m WorkspaceModel) updateInputScreen(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			m.screen = screenProjects
			if m.project != nil {
				m.screen = screenDashboard
			}
			m.input.Blur()
			return m, nil
		case "enter":
			value := strings.TrimSpace(m.input.Value())
			if value == "" {
				m.status = "⚠️ Informe um valor antes de continuar."
				return m, nil
			}
			switch m.screen {
			case screenNewProject:
				project, err := workspace.CreateProject(value)
				if err != nil {
					m.status = fmt.Sprintf("❌ %v", err)
					return m, nil
				}
				m.project = project
				m.reloadProjects()
				m.refreshFiles()
				m.screen = screenDashboard
				m.status = fmt.Sprintf("✅ Projeto '%s' criado.", project.Name)
			case screenInputPath:
				assets, err := m.project.ImportPaths([]string{value})
				if err != nil {
					m.status = fmt.Sprintf("❌ %v", err)
					return m, nil
				}
				m.refreshFiles()
				for _, asset := range assets {
					m.selected[asset.ID] = true
				}
				m.screen = screenDashboard
				m.status = fmt.Sprintf("✅ %d imagem(ns) importada(s) para o projeto.", len(assets))
			case screenGenerate:
				items := splitItems(value)
				if len(items) == 0 {
					m.status = "⚠️ Informe pelo menos um item."
					return m, nil
				}
				m.prepareRun("generate", items)
				m.input.Blur()
				return m, nil
			}
			m.input.Blur()
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m WorkspaceModel) updateDashboard(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "a":
		m.beginInput(screenInputPath, "Arquivo ou pasta de imagens", "./imagens")
	case "s":
		m.refreshFiles()
		if len(m.files) == 0 {
			m.status = "⚠️ Importe imagens antes de abrir a seleção."
		} else {
			m.screen = screenSelectImages
		}
	case "g":
		m.beginInput(screenGenerate, "Itens separados por vírgula", "maçã, banana, uva")
	case "c":
		m.prepareRun("colorize", nil)
	case "f":
		m.prepareRun("cards", nil)
	case "p":
		m.prepareRun("2up", nil)
	case "b", "esc":
		m.screen = screenProjects
		m.reloadProjects()
	}
	return m, nil
}

func (m WorkspaceModel) updateConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "enter", "y":
		return m.startRun(m.pendingOp, m.pendingItems)
	case "esc", "n":
		m.screen = screenDashboard
		m.pendingOp = ""
		m.pendingItems = nil
		m.status = "Operação não iniciada."
	}
	return m, nil
}

func (m WorkspaceModel) updateSelection(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.files)-1 {
			m.cursor++
		}
	case "space":
		if len(m.files) > 0 {
			id := m.files[m.cursor].ID
			m.selected[id] = !m.selected[id]
		}
	case "a":
		for _, file := range m.files {
			m.selected[file.ID] = true
		}
	case "n":
		for _, file := range m.files {
			m.selected[file.ID] = false
		}
	case "enter", "esc":
		m.screen = screenDashboard
	}
	return m, nil
}

func (m WorkspaceModel) startRun(operation string, items []string) (tea.Model, tea.Cmd) {
	if m.project == nil {
		m.status = "⚠️ Nenhum projeto aberto."
		return m, nil
	}
	if operation != "generate" && len(m.files) == 0 {
		m.status = "⚠️ Importe imagens antes de executar esta operação."
		return m, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan workspaceRunMessage, 32)
	project := m.project
	selectedIDs := m.selectedIDs()
	m.runCancel = cancel
	m.runEvents = nil
	m.status = ""
	m.screen = screenRunning
	go func() {
		service := workflow.ImageService{Project: project, Progress: func(event workflow.ProgressEvent) {
			ch <- workspaceRunMessage{event: event}
		}}
		var (
			artifacts []workspace.Artifact
			err       error
		)
		switch operation {
		case "generate":
			artifacts, err = service.Generate(ctx, workflow.ImageOptions{Items: items})
		case "colorize":
			artifacts, err = service.Colorize(ctx, selectedIDs, workflow.ImageOptions{})
		case "cards":
			artifacts, err = service.Cards(ctx, selectedIDs)
		case "2up":
			artifacts, err = service.TwoUp(ctx, selectedIDs)
		default:
			err = fmt.Errorf("operação desconhecida: %s", operation)
		}
		ch <- workspaceRunMessage{done: true, artifacts: artifacts, err: err}
		close(ch)
	}()
	return m, waitWorkspaceRun(ch)
}

func (m *WorkspaceModel) prepareRun(operation string, items []string) {
	if m.project == nil {
		m.status = "⚠️ Nenhum projeto aberto."
		return
	}
	if operation != "generate" && len(m.selectedIDs()) == 0 {
		m.status = "⚠️ Selecione pelo menos uma imagem antes de continuar."
		return
	}
	m.pendingOp = operation
	m.pendingItems = append([]string(nil), items...)
	m.screen = screenConfirm
	m.status = ""
}

func waitWorkspaceRun(ch <-chan workspaceRunMessage) tea.Cmd {
	return func() tea.Msg {
		message, ok := <-ch
		if !ok {
			return workspaceRunMessage{done: true}
		}
		message.channel = ch
		return message
	}
}

func (m *WorkspaceModel) beginInput(screen workspaceScreen, prompt, placeholder string) {
	m.screen = screen
	m.inputPrompt = prompt
	m.input.SetValue("")
	m.input.Placeholder = placeholder
	m.input.Focus()
	m.status = ""
}

func (m *WorkspaceModel) reloadProjects() {
	projects, err := workspace.ListProjects()
	if err != nil {
		m.fatal = err
		return
	}
	m.projects = projects
	if m.cursor >= len(projects) {
		m.cursor = len(projects) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *WorkspaceModel) refreshFiles() {
	if m.project == nil {
		return
	}
	m.files = m.project.ImageFiles()
	if m.cursor >= len(m.files) {
		m.cursor = len(m.files) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	if len(m.selected) == 0 {
		m.selected = make(map[string]bool, len(m.files))
	}
	for _, file := range m.files {
		if _, exists := m.selected[file.ID]; !exists {
			m.selected[file.ID] = true
		}
	}
}

func (m WorkspaceModel) selectedIDs() []string {
	var ids []string
	for _, file := range m.files {
		if m.selected[file.ID] {
			ids = append(ids, file.ID)
		}
	}
	return ids
}

func (m WorkspaceModel) View() string {
	if m.fatal != nil {
		return fmt.Sprintf("%s\n\n❌ %v\n", TitleStyle.Render("🍬 Caramel Workspace"), m.fatal)
	}
	var b strings.Builder
	b.WriteString(TitleStyle.Render("🍬 Caramel Workspace"))
	b.WriteString("\n")
	switch m.screen {
	case screenProjects:
		b.WriteString(HelpSectionTitleStyle.Render("Projetos"))
		b.WriteString("\n\n")
		if len(m.projects) == 0 {
			b.WriteString("Nenhum projeto ainda. Pressione n para criar o primeiro.\n")
		} else {
			for i, project := range m.projects {
				prefix := "  "
				style := UnselectedItemStyle
				if i == m.cursor {
					prefix = "› "
					style = SelectedItemStyle
				}
				b.WriteString(style.Render(prefix + project.Name))
				b.WriteString("  ")
				b.WriteString(HintStyle.Render(fmt.Sprintf("%d imagem(ns)", len(project.ImageFiles()))))
				b.WriteString("\n")
			}
		}
		b.WriteString("\n")
		b.WriteString(HintStyle.Render("↑/↓ navegar · enter abrir · n novo · r atualizar · q sair"))
	case screenNewProject, screenInputPath, screenGenerate:
		b.WriteString(HelpSectionTitleStyle.Render(m.inputPrompt))
		b.WriteString("\n\n")
		b.WriteString(m.input.View())
		b.WriteString("\n\n")
		b.WriteString(HintStyle.Render("enter confirmar · esc voltar"))
	case screenDashboard:
		b.WriteString(HelpSectionTitleStyle.Render(m.project.Name))
		b.WriteString("\n")
		b.WriteString(HintStyle.Render(m.project.Directory))
		b.WriteString("\n\n")
		b.WriteString(fmt.Sprintf("Imagens disponíveis: %d · Selecionadas: %d · Execuções: %d\n", len(m.files), len(m.selectedIDs()), len(m.project.Runs)))
		b.WriteString("\n")
		b.WriteString("a  adicionar arquivo/pasta\n")
		b.WriteString("s  selecionar imagens e visualizar\n")
		b.WriteString("g  gerar coleção com IA\n")
		b.WriteString("c  colorir selecionadas\n")
		b.WriteString("f  gerar fichas A4\n")
		b.WriteString("p  gerar PDF 2-up\n")
		b.WriteString("b  voltar aos projetos\n")
	case screenSelectImages:
		b.WriteString(HelpSectionTitleStyle.Render("Selecionar imagens"))
		b.WriteString("\n\n")
		if len(m.files) == 0 {
			b.WriteString("Nenhuma imagem disponível.\n")
		} else {
			for i, file := range m.files {
				check := "[ ]"
				if m.selected[file.ID] {
					check = "[x]"
				}
				style := UnselectedItemStyle
				prefix := "  "
				if i == m.cursor {
					style = SelectedItemStyle
					prefix = "› "
				}
				b.WriteString(style.Render(fmt.Sprintf("%s%s %s", prefix, check, file.Name)))
				b.WriteString("\n")
			}
			b.WriteString("\n")
			path := m.project.Resolve(m.files[m.cursor].Path)
			if preview, err := RenderImageFileToANSI(path, 34, 9); err == nil {
				b.WriteString(preview)
			}
		}
		b.WriteString("\n")
		b.WriteString(HintStyle.Render("↑/↓ navegar · espaço marcar · a todos · n nenhum · enter voltar"))
	case screenConfirm:
		b.WriteString(HelpSectionTitleStyle.Render("Confirmar operação"))
		b.WriteString("\n\n")
		b.WriteString(fmt.Sprintf("Projeto: %s\n", m.project.Name))
		b.WriteString(fmt.Sprintf("Operação: %s\n", operationLabel(m.pendingOp)))
		if m.pendingOp == "generate" {
			b.WriteString(fmt.Sprintf("Itens: %s\n", strings.Join(m.pendingItems, ", ")))
			b.WriteString(fmt.Sprintf("Modelo de imagem: %s\n", configuredModel(ai.DefaultModel, func(cfg config.Config) string { return cfg.ModelImage })))
			b.WriteString(fmt.Sprintf("Modelo de texto: %s\n", configuredModel(ai.DefaultTextModel, func(cfg config.Config) string { return cfg.ModelText })))
		} else if m.pendingOp == "colorize" {
			b.WriteString(fmt.Sprintf("Imagens selecionadas: %d\n", len(m.selectedIDs())))
			b.WriteString(fmt.Sprintf("Modelo de imagem: %s\n", configuredModel(ai.DefaultModel, func(cfg config.Config) string { return cfg.ModelImage })))
			b.WriteString(fmt.Sprintf("Modelo de triagem: %s\n", configuredModel(ai.DefaultTriageModel, func(cfg config.Config) string { return cfg.ModelTriage })))
		} else {
			b.WriteString(fmt.Sprintf("Imagens selecionadas: %d\n", len(m.selectedIDs())))
		}
		b.WriteString("Destino: workspace global do projeto\n")
		b.WriteString("\n")
		b.WriteString(HintStyle.Render("enter/y confirmar · esc/n cancelar"))
	case screenRunning:
		b.WriteString(HelpSectionTitleStyle.Render("Executando fluxo"))
		b.WriteString("\n\n")
		if len(m.runEvents) == 0 {
			b.WriteString("Preparando operação...\n")
		} else {
			for _, event := range m.runEvents {
				b.WriteString(formatWorkflowEvent(event))
				b.WriteString("\n")
			}
		}
		b.WriteString("\n")
		b.WriteString(HintStyle.Render("esc/c cancelar com segurança"))
	}
	if m.status != "" {
		b.WriteString("\n\n")
		b.WriteString(lipgloss.NewStyle().Foreground(ColorWarning).Render(m.status))
	}
	return b.String()
}

func splitItems(value string) []string {
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		if item := strings.TrimSpace(part); item != "" {
			items = append(items, item)
		}
	}
	return items
}

func operationLabel(operation string) string {
	switch operation {
	case "generate":
		return "Gerar imagens com IA"
	case "colorize":
		return "Colorir imagens com IA"
	case "cards":
		return "Gerar fichas A4"
	case "2up":
		return "Gerar PDF 2-up"
	default:
		return operation
	}
}

func configuredModel(fallback string, configured func(config.Config) string) string {
	cfg, err := config.LoadConfig()
	if err == nil {
		if value := configured(*cfg); value != "" {
			return value
		}
	}
	return fallback + " (padrão)"
}
