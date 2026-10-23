package shell

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// CompletionManager administra a integração persistente de completion em um
// perfil Bash, Zsh ou Fish. O comando gerado é chamado em cada nova sessão,
// portanto atualizações do executável não deixam uma cópia estática obsoleta.
type CompletionManager struct {
	profiles ProfileManager
}

// NewCompletionManager cria um gerenciador de completion para um perfil Linux.
func NewCompletionManager(homeDir, shellName, zdotdir, xdgConfigHome string) CompletionManager {
	return CompletionManager{
		profiles: NewProfileManager(homeDir, shellName, zdotdir, xdgConfigHome, "unused"),
	}
}

// Status informa se o perfil está ausente, sem configuração, ativo ou inválido.
func (m CompletionManager) Status() ([]ProfileState, error) {
	profile, err := m.profiles.Profile()
	if err != nil {
		return nil, err
	}
	contentsBytes, err := os.ReadFile(profile.Path)
	if errors.Is(err, os.ErrNotExist) {
		return []ProfileState{{Profile: profile, State: "absent"}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ler perfil %s: %w", profile.Path, err)
	}
	contents := string(contentsBytes)
	start, end := completionMarkers(m.profiles.Shell)
	_, _, found, markerErr := managedBlockRangeFor(contents, start, end)
	if markerErr != nil {
		return []ProfileState{{Profile: profile, State: "invalid"}}, nil
	}
	if found {
		return []ProfileState{{Profile: profile, State: "active"}}, nil
	}
	return []ProfileState{{Profile: profile, State: "not_configured"}}, nil
}

// Enable instala ou atualiza somente o bloco gerenciado de completion.
func (m CompletionManager) Enable() ([]ProfileState, error) {
	profile, err := m.profiles.Profile()
	if err != nil {
		return nil, err
	}
	contents, err := readOptional(profile.Path)
	if err != nil {
		return nil, err
	}
	start, end := completionMarkers(m.profiles.Shell)
	if _, _, _, markerErr := managedBlockRangeFor(contents, start, end); markerErr != nil {
		return nil, fmt.Errorf("perfil %s: %w", profile.Path, markerErr)
	}
	updated, err := installManagedBlockFor(contents, start, end, completionBlock(m.profiles.Shell, lineEndingFor(contents)))
	if err != nil {
		return nil, fmt.Errorf("atualizar perfil %s: %w", profile.Path, err)
	}
	if updated != contents {
		if err := writeFileAtomic(profile.Path, []byte(updated)); err != nil {
			return nil, err
		}
	}
	return m.Status()
}

// Disable remove somente o bloco gerenciado e preserva o restante do perfil.
func (m CompletionManager) Disable() ([]ProfileState, error) {
	profile, err := m.profiles.Profile()
	if err != nil {
		return nil, err
	}
	contents, err := readOptional(profile.Path)
	if err != nil {
		return nil, err
	}
	start, end := completionMarkers(m.profiles.Shell)
	updated, err := removeManagedBlockFor(contents, start, end)
	if err != nil {
		return nil, fmt.Errorf("atualizar perfil %s: %w", profile.Path, err)
	}
	if updated != contents {
		if err := writeFileAtomic(profile.Path, []byte(updated)); err != nil {
			return nil, err
		}
	}
	return m.Status()
}

func completionMarkers(shellName string) (string, string) {
	shellName = normalizeShellName(shellName)
	return fmt.Sprintf("# >>> caramel completion %s >>>", shellName), fmt.Sprintf("# <<< caramel completion %s <<<", shellName)
}

func completionBlock(shellName, lineEnding string) string {
	shellName = normalizeShellName(shellName)
	start, end := completionMarkers(shellName)
	lines := []string{start}
	switch shellName {
	case "bash":
		lines = append(lines,
			"if command -v caramel >/dev/null 2>&1; then",
			"    eval \"$(caramel completion bash 2>/dev/null)\"",
			"fi",
		)
	case "zsh":
		lines = append(lines,
			"if (( $+commands[caramel] )); then",
			"    eval \"$(caramel completion zsh 2>/dev/null)\"",
			"fi",
		)
	case "fish":
		lines = append(lines,
			"if type -q caramel",
			"    caramel completion fish 2>/dev/null | source",
			"end",
		)
	default:
		return strings.Join([]string{start, end}, lineEnding) + lineEnding
	}
	lines = append(lines, end)
	return strings.Join(lines, lineEnding) + lineEnding
}
