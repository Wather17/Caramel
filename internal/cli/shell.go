package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"caramel/internal/output"
	"caramel/internal/shell"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

var (
	currentOperatingSystem = runtime.GOOS
	currentUserHomeDir     = os.UserHomeDir
	interactiveInput       = isInteractiveInput
)

var shellCmd = &cobra.Command{
	Use:   "shell",
	Short: "Configura integrações opcionais com o terminal",
	Long:  "Ativa, verifica ou remove integrações gerenciadas do Caramel com shells do usuário.",
}

var shellEnableCmd = &cobra.Command{
	Use:   "enable <shell>",
	Short: "Ativa uma integração de shell",
	Long: `Instala um bloco gerenciado de autocomplete nos perfis PowerShell 7 e Windows PowerShell 5.1.
O restante dos perfis é preservado e uma nova execução apenas atualiza o mesmo bloco.

📚 QUANDO USAR:
Use depois de instalar o Caramel ou quando quiser ativar Tab para buscar arquivos indexados de qualquer pasta.`,
	Example: `# Ativar autocomplete permanente nos perfis PowerShell
caramel shell enable powershell`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		manager, err := currentPowerShellManager(args[0])
		if err != nil {
			return err
		}
		states, err := manager.Enable()
		if err != nil {
			return err
		}
		return renderPowerShellStates(cmd, states, "Integração do PowerShell ativada")
	},
}

var shellStatusCmd = &cobra.Command{
	Use:   "status <shell>",
	Short: "Mostra o estado de uma integração de shell",
	Long: `Informa quais perfis PowerShell do usuário estão ausentes, não configurados ou com o bloco do Caramel ativo.

📚 QUANDO USAR:
Use para confirmar se o autocomplete está disponível antes de abrir uma nova janela do terminal.`,
	Example: `# Verificar a integração do PowerShell
caramel shell status powershell`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		manager, err := currentPowerShellManager(args[0])
		if err != nil {
			return err
		}
		states, err := manager.Status()
		if err != nil {
			return err
		}
		return renderPowerShellStates(cmd, states, "Estado da integração do PowerShell")
	},
}

var shellDisableCmd = &cobra.Command{
	Use:   "disable <shell>",
	Short: "Remove uma integração gerenciada de shell",
	Long: `Remove somente o bloco delimitado que o Caramel instalou nos perfis PowerShell.
Linhas e personalizações externas permanecem intactas.

📚 QUANDO USAR:
Use se quiser desativar o autocomplete permanente sem apagar ou editar o perfil manualmente.`,
	Example: `# Remover a integração gerenciada do PowerShell
caramel shell disable powershell`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		manager, err := currentPowerShellManager(args[0])
		if err != nil {
			return err
		}
		states, err := manager.Disable()
		if err != nil {
			return err
		}
		return renderPowerShellStates(cmd, states, "Integração do PowerShell removida")
	},
}

func currentPowerShellManager(shellName string) (shell.PowerShellManager, error) {
	if !strings.EqualFold(strings.TrimSpace(shellName), "powershell") {
		return shell.PowerShellManager{}, fmt.Errorf("shell não suportado: %s (use powershell)", shellName)
	}
	homeDir, err := currentUserHomeDir()
	if err != nil {
		return shell.PowerShellManager{}, fmt.Errorf("localizar diretório do usuário: %w", err)
	}
	return shell.NewPowerShellManager(homeDir, currentOperatingSystem), nil
}

func renderPowerShellStates(cmd *cobra.Command, states []shell.ProfileState, summary string) error {
	renderer, err := output.New(outputOptions(), cmd.OutOrStdout(), cmd.ErrOrStderr())
	if err != nil {
		return err
	}
	active := 0
	outputs := make([]string, 0, len(states))
	for _, state := range states {
		if state.State == "active" {
			active++
		}
		outputs = append(outputs, state.Path)
		if err := renderer.Text("%-24s %s · %s\n", state.Name, state.State, state.Path); err != nil {
			return err
		}
	}
	return renderer.Result(output.Result{
		Status:  output.StateSuccess,
		Summary: fmt.Sprintf("%s: %d/%d perfil(is) ativo(s).", summary, active, len(states)),
		Count:   active,
		Data:    states,
		Outputs: outputs,
	})
}

func shouldOfferPowerShellIntegration(cmd *cobra.Command, renderer *output.Renderer) bool {
	if currentOperatingSystem != "windows" || renderer.Options().JSON || renderer.Options().Quiet {
		return false
	}
	return interactiveInput(cmd.InOrStdin())
}

func offerPowerShellIntegration(cmd *cobra.Command, renderer *output.Renderer) error {
	if !shouldOfferPowerShellIntegration(cmd, renderer) {
		return nil
	}
	manager, err := currentPowerShellManager("powershell")
	if err != nil {
		return err
	}
	states, err := manager.Status()
	if err != nil {
		return err
	}
	for _, state := range states {
		if state.State != "active" {
			return promptPowerShellIntegration(cmd, renderer, manager)
		}
	}
	return nil
}

func promptPowerShellIntegration(cmd *cobra.Command, renderer *output.Renderer, manager shell.PowerShellManager) error {
	if err := renderer.Text("Ativar autocomplete do vault no PowerShell? [S/n] "); err != nil {
		return err
	}
	answer, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if err != nil && err != io.EOF {
		return fmt.Errorf("ler confirmação da integração PowerShell: %w", err)
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	if answer != "" && answer != "s" && answer != "sim" && answer != "y" && answer != "yes" {
		return renderer.Text("Autocomplete do PowerShell não foi ativado. Use `caramel shell enable powershell` quando quiser.\n")
	}
	states, err := manager.Enable()
	if err != nil {
		return err
	}
	active := 0
	for _, state := range states {
		if state.State == "active" {
			active++
		}
	}
	return renderer.Text("Autocomplete do vault ativado em %d perfil(is) do PowerShell. Abra uma nova janela para usar Tab.\n", active)
}

func isInteractiveInput(reader io.Reader) bool {
	file, ok := reader.(*os.File)
	if !ok {
		return false
	}
	return isatty.IsTerminal(file.Fd()) || isatty.IsCygwinTerminal(file.Fd())
}

func init() {
	shellCmd.AddCommand(shellEnableCmd, shellStatusCmd, shellDisableCmd)
	RootCmd.AddCommand(shellCmd)
}
