package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"caramel/internal/output"
	"caramel/internal/shell"

	"github.com/spf13/cobra"
)

var selfInstallCmd = &cobra.Command{
	Use:     "install",
	Aliases: []string{"self-install"},
	Short:   "Instala o Caramel CLI globalmente no sistema",
	Long: `Copia o binário do Caramel em execução para um diretório local do usuário
e adiciona esse diretório ao PATH do sistema operacional.

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
		if warning, err := installMelAlias(installDir, binaryName); err != nil {
			warnings = append(warnings, err.Error())
		} else if warning != "" {
			warnings = append(warnings, warning)
		}

		// 4. Configura o PATH com base no Sistema Operacional
		if runtime.GOOS == "windows" {
			err := addPathWindows(installDir)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("não foi possível atualizar o PATH automaticamente; adicione %s ao PATH do Windows", installDir))
				renderer.Diagnostic("falha ao atualizar PATH: %v\n", err)
			} else {
				warnings = append(warnings, "reinicie o terminal ou PowerShell para atualizar o PATH")
			}
		} else if runtime.GOOS == "linux" {
			update, err := configureLinuxPath(
				homeDir,
				installDir,
				os.Getenv("PATH"),
				os.Getenv("SHELL"),
				os.Getenv("ZDOTDIR"),
				os.Getenv("XDG_CONFIG_HOME"),
			)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("não foi possível atualizar o PATH automaticamente; adicione %s ao PATH e reabra o terminal: %v", installDir, err))
				renderer.Diagnostic("falha ao atualizar PATH: %v\n", err)
			} else if update.Updated {
				warnings = append(warnings, fmt.Sprintf("PATH atualizado em %s; abra um novo terminal ou execute: source %s", update.Profile.Path, update.Profile.Path))
			} else {
				renderer.Diagnostic("diretório de instalação já está configurado no PATH\n")
			}
		} else {
			// macOS mantém o comportamento anterior: orientar o usuário sem
			// editar automaticamente os perfis desta plataforma.
			if !shell.ContainsPath(os.Getenv("PATH"), installDir) {
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

func configureLinuxPath(homeDir, installDir, pathEnv, shellName, zdotdir, xdgConfigHome string) (shell.PathUpdate, error) {
	manager := shell.NewProfileManager(homeDir, shellName, zdotdir, xdgConfigHome, installDir)
	profile, err := manager.Profile()
	if err != nil {
		return shell.PathUpdate{}, err
	}
	if shell.ContainsPath(pathEnv, installDir) {
		return shell.PathUpdate{Profile: profile, AlreadyConfigured: true}, nil
	}
	return manager.EnsurePath()
}

// copyFile copia o conteúdo do arquivo src para o arquivo dst
func copyFile(src, dst string) error {
	if samePath(src, dst) {
		return nil
	}

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

func samePath(left, right string) bool {
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	return filepath.Clean(leftAbs) == filepath.Clean(rightAbs)
}

const managedWindowsMelLauncher = "REM Caramel managed launcher for mel"

// installMelAlias cria o atalho curto sem substituir arquivos que o usuário
// já possua no mesmo diretório.
func installMelAlias(installDir, binaryName string) (string, error) {
	aliasName := "mel"
	if runtime.GOOS == "windows" {
		aliasName = "mel.cmd"
	}
	aliasPath := filepath.Join(installDir, aliasName)

	info, err := os.Lstat(aliasPath)
	if errors.Is(err, os.ErrNotExist) {
		if runtime.GOOS == "windows" {
			content := fmt.Sprintf("@echo off\r\n%s\r\n\"%%~dp0%s\" %%*\r\nexit /b %%ERRORLEVEL%%\r\n", managedWindowsMelLauncher, binaryName)
			if err := os.WriteFile(aliasPath, []byte(content), 0644); err != nil {
				return "", fmt.Errorf("criar alias %s: %w", aliasName, err)
			}
		} else if err := os.Symlink(binaryName, aliasPath); err != nil {
			return "", fmt.Errorf("criar alias %s: %w", aliasName, err)
		}
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("inspecionar alias %s: %w", aliasName, err)
	}

	if runtime.GOOS == "windows" {
		if info.IsDir() {
			return fmt.Sprintf("o caminho '%s' já existe como diretório; mantido sem alteração", aliasPath), nil
		}
		contents, readErr := os.ReadFile(aliasPath)
		if readErr != nil {
			return "", fmt.Errorf("ler alias %s: %w", aliasName, readErr)
		}
		if !strings.Contains(string(contents), managedWindowsMelLauncher) {
			return fmt.Sprintf("o arquivo '%s' já existe e não é gerenciado pelo Caramel; mantido sem alteração", aliasPath), nil
		}
		content := fmt.Sprintf("@echo off\r\n%s\r\n\"%%~dp0%s\" %%*\r\nexit /b %%ERRORLEVEL%%\r\n", managedWindowsMelLauncher, binaryName)
		if err := os.WriteFile(aliasPath, []byte(content), 0644); err != nil {
			return "", fmt.Errorf("atualizar alias %s: %w", aliasName, err)
		}
		return "", nil
	}

	if info.Mode()&os.ModeSymlink != 0 {
		target, readErr := os.Readlink(aliasPath)
		if readErr != nil {
			return "", fmt.Errorf("ler alias %s: %w", aliasName, readErr)
		}
		if target == binaryName || target == filepath.Join(installDir, binaryName) {
			return "", nil
		}
	}
	return fmt.Sprintf("o caminho '%s' já existe e não é gerenciado pelo Caramel; mantido sem alteração", aliasPath), nil
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
