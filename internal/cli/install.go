package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

var selfInstallCmd = &cobra.Command{
	Use:     "install",
	Aliases: []string{"self-install"},
	Short:   "Instala o Caramel CLI globalmente no sistema",
	Long: `Copia o binário do Caramel em execução para um diretório local do usuário
e adiciona esse diretório ao PATH do sistema operacional (automaticamente no Windows e orientações no Linux).`,
	RunE: func(cmd *cobra.Command, args []string) error {
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

		fmt.Printf("🍬 Instalando Caramel CLI para %s...\n", runtime.GOOS)

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

		fmt.Printf(" ├─ Binário copiado para: %s\n", targetPath)

		// 4. Configura o PATH com base no Sistema Operacional
		if runtime.GOOS == "windows" {
			err := addPathWindows(installDir)
			if err != nil {
				fmt.Printf(" ⚠️  Não foi possível configurar o PATH automaticamente: %v\n", err)
				fmt.Println("    Por favor, adicione o caminho acima manualmente ao PATH do Windows.")
			} else {
				fmt.Println(" ├─ Diretório adicionado ao PATH de Usuário no Registro do Windows com sucesso!")
				fmt.Println("    (Reinicie o terminal ou PowerShell para atualizar as variáveis de ambiente)")
			}
		} else {
			// Linux / macOS
			pathEnv := os.Getenv("PATH")
			if !strings.Contains(pathEnv, installDir) {
				fmt.Printf("\n📢  Certifique-se de adicionar '%s' ao seu PATH!\n", installDir)
				fmt.Println("    Adicione a seguinte linha ao seu ~/.bashrc ou ~/.zshrc:")
				fmt.Printf("    👉 export PATH=\"$HOME/.local/bin:$PATH\"\n\n")
			} else {
				fmt.Println(" ├─ O diretório de instalação já está presente no seu PATH!")
			}
		}

		fmt.Println("✅ Instalação do Caramel concluída com sucesso!")
		return nil
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
