package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"caramel/internal/output"

	"github.com/spf13/cobra"
)

var selfInstallCmd = &cobra.Command{
	Use:     "install",
	Aliases: []string{"self-install"},
	Short:   "Instala o Caramel CLI globalmente no sistema",
	Long: `Copia o binário do Caramel em execução para um diretório local do usuário
e adiciona esse diretório ao PATH do sistema operacional (automaticamente no Windows e orientações no Linux).

📚 QUANDO USAR:
Use após baixar o binário para instalá-lo globalmente: o comando copia o executável para um
diretório local do usuário e configura o PATH automaticamente, permitindo usar 'caramel' de qualquer pasta.`,
	Example: `# Executar o auto-instalador e configurar o PATH global
caramel install`,
	RunE: func(cmd *cobra.Command, args []string) error {
		renderer, err := output.New(outputOptions(), cmd.OutOrStdout(), cmd.ErrOrStderr())
		if err != nil {
			return err
		}
		// 1. Obtém o caminho do executável atual
		exePath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("não foi possível obter o caminho do executável: %w", err)
		}

		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("não foi possível obter o diretório home do usuário: %w", err)
		}

		var installDir string
		var targetPath string
		binaryName := "caramel"

		if runtime.GOOS == "windows" {
			binaryName = "caramel.exe"
			installDir = filepath.Join(homeDir, ".caramel", "bin")
			targetPath = filepath.Join(installDir, binaryName)
		} else {
			installDir = filepath.Join(homeDir, ".local", "bin")
			targetPath = filepath.Join(installDir, binaryName)
		}

		renderer.Diagnostic("instalando Caramel CLI para %s\n", runtime.GOOS)

		// 2. Cria a pasta de instalação caso ela não exista
		if err := os.MkdirAll(installDir, 0755); err != nil {
			return fmt.Errorf("falha ao criar pasta de instalação '%s': %w", installDir, err)
		}

		// 3. Copia o executável atual para a pasta de destino
		if err := copyFile(exePath, targetPath); err != nil {
			return fmt.Errorf("falha ao copiar executável para a pasta de destino: %w", err)
		}

		// Garante permissões de execução no Linux/macOS
		if runtime.GOOS != "windows" {
			if err := os.Chmod(targetPath, 0755); err != nil {
				return fmt.Errorf("falha ao aplicar permissões de execução: %w", err)
			}
		}

		renderer.Diagnostic("binário copiado para: %s\n", targetPath)
		warnings := []string{}

		// 4. Configura o PATH com base no Sistema Operacional
		if runtime.GOOS == "windows" {
			err := addPathWindows(installDir)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("não foi possível atualizar o PATH automaticamente; adicione %s ao PATH do Windows", installDir))
				renderer.Diagnostic("falha ao atualizar PATH: %v\n", err)
			} else {
				warnings = append(warnings, "reinicie o terminal ou PowerShell para atualizar o PATH")
			}
		} else {
			// Linux / macOS
			pathEnv := os.Getenv("PATH")
			if !strings.Contains(pathEnv, installDir) {
				warnings = append(warnings, fmt.Sprintf("adicione %s ao PATH e reabra o terminal", installDir))
			} else {
				renderer.Diagnostic("diretório de instalação já está no PATH\n")
			}
		}

		return renderer.Result(output.Result{
			Status:   output.StateSuccess,
			Summary:  "Caramel instalado com sucesso.",
			Outputs:  []string{targetPath},
			Warnings: warnings,
		})
	},
}

// copyFile copia o conteúdo do arquivo src para o arquivo dst
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	// Se o arquivo de destino já existe, tenta remover para evitar travamentos caso esteja em uso
	_ = os.Remove(dst)

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

// addPathWindows altera a variável PATH de usuário no registro do Windows usando PowerShell
func addPathWindows(installDir string) error {
	// Commando PowerShell para buscar e atualizar o PATH de Usuário
	psCommand := fmt.Sprintf(
		`$UserPath = [Environment]::GetEnvironmentVariable("PATH", "User"); `+
			`if ($UserPath -notlike "*%s*") { `+
			`  [Environment]::SetEnvironmentVariable("PATH", "$UserPath;%s", "User") `+
			`}`,
		installDir, installDir,
	)

	cmd := exec.Command("powershell", "-Command", psCommand)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("powershell executou com erro: %w, output: %s", err, string(output))
	}
	return nil
}

func init() {
	RootCmd.AddCommand(selfInstallCmd)
}
