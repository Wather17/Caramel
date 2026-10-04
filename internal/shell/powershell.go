// Package shell gerencia integrações opcionais da CLI com shells do usuário.
package shell

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	powerShellBlockStart = "# >>> caramel powershell completion >>>"
	powerShellBlockEnd   = "# <<< caramel powershell completion <<<"
)

var ErrPowerShellUnsupported = errors.New("a integração do PowerShell está disponível apenas no Windows")

// Profile representa um perfil de PowerShell que pode receber a integração.
type Profile struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// ProfileState descreve o estado do bloco gerenciado em um perfil.
type ProfileState struct {
	Profile
	State string `json:"state"`
}

// PowerShellManager administra somente o bloco delimitado pelo Caramel nos perfis do usuário.
// OS permite testar o comportamento Windows sem escrever no perfil real.
type PowerShellManager struct {
	HomeDir string
	OS      string
}

func NewPowerShellManager(homeDir, operatingSystem string) PowerShellManager {
	return PowerShellManager{HomeDir: homeDir, OS: operatingSystem}
}

// Profiles retorna os perfis atuais de PowerShell 7 e Windows PowerShell 5.1.
func (m PowerShellManager) Profiles() ([]Profile, error) {
	if !strings.EqualFold(m.OS, "windows") {
		return nil, ErrPowerShellUnsupported
	}
	if strings.TrimSpace(m.HomeDir) == "" {
		return nil, fmt.Errorf("diretório do usuário não informado")
	}
	documents := filepath.Join(m.HomeDir, "Documents")
	return []Profile{
		{Name: "PowerShell 7", Path: filepath.Join(documents, "PowerShell", "Microsoft.PowerShell_profile.ps1")},
		{Name: "Windows PowerShell 5.1", Path: filepath.Join(documents, "WindowsPowerShell", "Microsoft.PowerShell_profile.ps1")},
	}, nil
}

// Status informa se cada perfil está ausente, não configurado ou com a integração ativa.
func (m PowerShellManager) Status() ([]ProfileState, error) {
	profiles, err := m.Profiles()
	if err != nil {
		return nil, err
	}
	states := make([]ProfileState, 0, len(profiles))
	for _, profile := range profiles {
		contents, err := os.ReadFile(profile.Path)
		if errors.Is(err, os.ErrNotExist) {
			states = append(states, ProfileState{Profile: profile, State: "absent"})
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("ler perfil %s: %w", profile.Path, err)
		}
		if _, _, found, err := managedBlockRange(string(contents)); err != nil {
			return nil, fmt.Errorf("perfil %s: %w", profile.Path, err)
		} else if found {
			states = append(states, ProfileState{Profile: profile, State: "active"})
		} else {
			states = append(states, ProfileState{Profile: profile, State: "not_configured"})
		}
	}
	return states, nil
}

// Enable instala ou atualiza o bloco do Caramel sem alterar linhas não gerenciadas.
func (m PowerShellManager) Enable() ([]ProfileState, error) {
	profiles, err := m.Profiles()
	if err != nil {
		return nil, err
	}
	for _, profile := range profiles {
		contents, err := readOptional(profile.Path)
		if err != nil {
			return nil, err
		}
		updated, err := installManagedBlock(contents)
		if err != nil {
			return nil, fmt.Errorf("atualizar perfil %s: %w", profile.Path, err)
		}
		if updated != contents {
			if err := writeFileAtomic(profile.Path, []byte(updated)); err != nil {
				return nil, err
			}
		}
	}
	return m.Status()
}

// Disable remove somente o bloco que o Caramel gerencia. Repetir a operação é seguro.
func (m PowerShellManager) Disable() ([]ProfileState, error) {
	profiles, err := m.Profiles()
	if err != nil {
		return nil, err
	}
	for _, profile := range profiles {
		contents, err := readOptional(profile.Path)
		if err != nil {
			return nil, err
		}
		updated, err := removeManagedBlock(contents)
		if err != nil {
			return nil, fmt.Errorf("atualizar perfil %s: %w", profile.Path, err)
		}
		if updated != contents {
			if err := writeFileAtomic(profile.Path, []byte(updated)); err != nil {
				return nil, err
			}
		}
	}
	return m.Status()
}

func readOptional(path string) (string, error) {
	contents, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("ler perfil %s: %w", path, err)
	}
	return string(contents), nil
}

func installManagedBlock(contents string) (string, error) {
	start, end, found, err := managedBlockRange(contents)
	if err != nil {
		return "", err
	}
	block := powerShellBlock(lineEndingFor(contents))
	if found {
		return contents[:start] + block + contents[end:], nil
	}
	if contents == "" {
		return block, nil
	}
	return contents + lineEndingFor(contents) + block, nil
}

func removeManagedBlock(contents string) (string, error) {
	start, end, found, err := managedBlockRange(contents)
	if err != nil || !found {
		return contents, err
	}
	if start >= 2 && contents[start-2:start] == "\r\n" {
		start -= 2
	} else if start >= 1 && contents[start-1:start] == "\n" {
		start--
	}
	return contents[:start] + contents[end:], nil
}

func managedBlockRange(contents string) (int, int, bool, error) {
	starts := strings.Count(contents, powerShellBlockStart)
	ends := strings.Count(contents, powerShellBlockEnd)
	if starts == 0 && ends == 0 {
		return 0, 0, false, nil
	}
	if starts != 1 || ends != 1 {
		return 0, 0, false, fmt.Errorf("marcadores gerenciados inválidos; remova ou repare o bloco do Caramel manualmente")
	}
	start := strings.Index(contents, powerShellBlockStart)
	endStart := strings.Index(contents[start:], powerShellBlockEnd)
	if endStart < 0 {
		return 0, 0, false, fmt.Errorf("marcador final ausente")
	}
	end := start + endStart + len(powerShellBlockEnd)
	if strings.HasPrefix(contents[end:], "\r\n") {
		end += 2
	} else if strings.HasPrefix(contents[end:], "\n") {
		end++
	}
	return start, end, true, nil
}

func lineEndingFor(contents string) string {
	if strings.Contains(contents, "\r\n") {
		return "\r\n"
	}
	return "\n"
}

func powerShellBlock(lineEnding string) string {
	lines := []string{
		powerShellBlockStart,
		"# Mantem a saida nativa e os caminhos indexados em UTF-8.",
		"$caramelUtf8Encoding = [System.Text.UTF8Encoding]::new($false)",
		"[Console]::InputEncoding = $caramelUtf8Encoding",
		"[Console]::OutputEncoding = $caramelUtf8Encoding",
		"$OutputEncoding = $caramelUtf8Encoding",
		"",
		"if (Get-Command -Name caramel -ErrorAction SilentlyContinue) {",
		"    caramel completion powershell | Out-String | Invoke-Expression",
		"",
		"    # Insere caminhos com espacos como um unico argumento legivel.",
		"    filter __caramel_escapeStringWithSpecialChars {",
		"        if ($_ -match '[\\s\\\\]') {",
		"            $quote = [string][char]39",
		"            $quote + $_.Replace($quote, $quote + $quote) + $quote",
		"        } else {",
		"            $_",
		"        }",
		"    }",
		"}",
		powerShellBlockEnd,
	}
	return strings.Join(lines, lineEnding) + lineEnding
}

func writeFileAtomic(path string, contents []byte) error {
	mode := os.FileMode(0o600)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspecionar perfil %s: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("criar diretório do perfil %s: %w", path, err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".caramel-profile-*")
	if err != nil {
		return fmt.Errorf("criar arquivo temporário para %s: %w", path, err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(mode); err != nil {
		temporary.Close()
		return fmt.Errorf("definir permissões do perfil temporário: %w", err)
	}
	if _, err := temporary.Write(contents); err != nil {
		temporary.Close()
		return fmt.Errorf("gravar perfil temporário: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("fechar perfil temporário: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("substituir perfil %s: %w", path, err)
	}
	return nil
}
