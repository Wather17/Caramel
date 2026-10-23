package shell

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

const (
	pathBlockStart = "# >>> caramel path >>>"
	pathBlockEnd   = "# <<< caramel path <<<"
)

// ProfileManager administra o bloco de PATH do perfil do shell do usuário.
// Os diretórios de configuração são recebidos explicitamente para manter o
// comportamento testável e respeitar ZDOTDIR/XDG_CONFIG_HOME.
type ProfileManager struct {
	HomeDir       string
	Shell         string
	ZDOTDIR       string
	XDGConfigHome string
	InstallDir    string
}

// PathUpdate descreve o resultado da tentativa de configurar o PATH.
type PathUpdate struct {
	Profile           Profile
	Updated           bool
	AlreadyConfigured bool
}

// NewProfileManager cria um gerenciador de perfil para um shell suportado.
func NewProfileManager(homeDir, shellName, zdotdir, xdgConfigHome, installDir string) ProfileManager {
	return ProfileManager{
		HomeDir:       homeDir,
		Shell:         shellName,
		ZDOTDIR:       zdotdir,
		XDGConfigHome: xdgConfigHome,
		InstallDir:    installDir,
	}
}

// Profile retorna o arquivo de inicialização correspondente ao shell.
func (m ProfileManager) Profile() (Profile, error) {
	homeDir := strings.TrimSpace(m.HomeDir)
	if homeDir == "" {
		return Profile{}, fmt.Errorf("diretório do usuário não informado")
	}

	shellName := normalizeShellName(m.Shell)
	switch shellName {
	case "bash":
		return Profile{Name: "Bash", Path: filepath.Join(homeDir, ".bashrc")}, nil
	case "zsh":
		profileDir := strings.TrimSpace(m.ZDOTDIR)
		if profileDir == "" {
			profileDir = homeDir
		}
		return Profile{Name: "Zsh", Path: filepath.Join(profileDir, ".zshrc")}, nil
	case "fish":
		configDir := strings.TrimSpace(m.XDGConfigHome)
		if configDir == "" {
			configDir = filepath.Join(homeDir, ".config")
		}
		return Profile{Name: "Fish", Path: filepath.Join(configDir, "fish", "config.fish")}, nil
	default:
		return Profile{}, fmt.Errorf("shell não suportado: %s (use bash, zsh ou fish)", m.Shell)
	}
}

// EnsurePath instala ou atualiza somente o bloco de PATH gerenciado pelo Caramel.
func (m ProfileManager) EnsurePath() (PathUpdate, error) {
	if strings.TrimSpace(m.InstallDir) == "" {
		return PathUpdate{}, fmt.Errorf("diretório de instalação não informado")
	}
	profile, err := m.Profile()
	if err != nil {
		return PathUpdate{}, err
	}

	contents, err := readOptional(profile.Path)
	if err != nil {
		return PathUpdate{}, err
	}
	if _, _, found, err := managedBlockRangeFor(contents, pathBlockStart, pathBlockEnd); err != nil {
		return PathUpdate{}, fmt.Errorf("perfil %s: %w", profile.Path, err)
	} else if found {
		updated, err := installManagedBlockFor(contents, pathBlockStart, pathBlockEnd, m.pathBlock(lineEndingFor(contents)))
		if err != nil {
			return PathUpdate{}, fmt.Errorf("atualizar perfil %s: %w", profile.Path, err)
		}
		if updated != contents {
			if err := writeFileAtomic(profile.Path, []byte(updated)); err != nil {
				return PathUpdate{}, err
			}
			return PathUpdate{Profile: profile, Updated: true}, nil
		}
		return PathUpdate{Profile: profile, AlreadyConfigured: true}, nil
	}

	if profileContainsPath(contents, normalizeShellName(m.Shell), m.HomeDir, m.InstallDir) {
		return PathUpdate{Profile: profile, AlreadyConfigured: true}, nil
	}

	updated, err := installManagedBlockFor(contents, pathBlockStart, pathBlockEnd, m.pathBlock(lineEndingFor(contents)))
	if err != nil {
		return PathUpdate{}, fmt.Errorf("atualizar perfil %s: %w", profile.Path, err)
	}
	if err := writeFileAtomic(profile.Path, []byte(updated)); err != nil {
		return PathUpdate{}, err
	}
	return PathUpdate{Profile: profile, Updated: true}, nil
}

// ContainsPath verifica entradas exatas de uma variável PATH, sem aceitar
// caminhos que apenas contenham o diretório como substring.
func ContainsPath(pathValue, target string) bool {
	target = filepath.Clean(strings.TrimSpace(target))
	if target == "." || target == "" {
		return false
	}
	for _, entry := range filepath.SplitList(pathValue) {
		entry = strings.TrimSpace(entry)
		if entry != "" && filepath.Clean(entry) == target {
			return true
		}
	}
	return false
}

func (m ProfileManager) pathBlock(lineEnding string) string {
	path := shellSingleQuote(m.InstallDir)
	lines := []string{pathBlockStart}
	if normalizeShellName(m.Shell) == "fish" {
		lines = append(lines, fmt.Sprintf("set -gx PATH %s $PATH", path))
	} else {
		lines = append(lines, fmt.Sprintf("export PATH=%s:$PATH", path))
	}
	lines = append(lines, pathBlockEnd)
	return strings.Join(lines, lineEnding) + lineEnding
}

func normalizeShellName(shellName string) string {
	shellName = strings.TrimSpace(shellName)
	if shellName == "" {
		return ""
	}
	return strings.ToLower(filepath.Base(shellName))
}

// IsSupportedShell reports whether a profile manager can target the shell.
func IsSupportedShell(shellName string) bool {
	switch normalizeShellName(shellName) {
	case "bash", "zsh", "fish":
		return true
	default:
		return false
	}
}

func profileContainsPath(contents, shellName, homeDir, installDir string) bool {
	for _, rawLine := range strings.Split(contents, "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(rawLine, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		words := shellWords(line)
		if shellName == "fish" {
			if len(words) == 0 || words[0] != "set" {
				continue
			}
			for index, word := range words {
				if word != "PATH" {
					continue
				}
				for _, entry := range words[index+1:] {
					if pathEntryEquivalent(entry, homeDir, installDir) {
						return true
					}
				}
				break
			}
			continue
		}

		if !strings.HasPrefix(line, "export PATH=") && !strings.HasPrefix(line, "PATH=") {
			continue
		}
		assignment := line[strings.IndexByte(line, '=')+1:]
		for _, word := range shellWords(assignment) {
			for _, entry := range strings.Split(word, ":") {
				if pathEntryEquivalent(entry, homeDir, installDir) {
					return true
				}
			}
		}
	}
	return false
}

func pathEntryEquivalent(entry, homeDir, installDir string) bool {
	entry = strings.TrimSpace(strings.Trim(entry, "\"'"))
	if entry == "" || entry == "$PATH" || entry == "${PATH}" {
		return false
	}
	if strings.HasPrefix(entry, "$HOME/") {
		entry = filepath.Join(homeDir, strings.TrimPrefix(entry, "$HOME/"))
	} else if strings.HasPrefix(entry, "${HOME}/") {
		entry = filepath.Join(homeDir, strings.TrimPrefix(entry, "${HOME}/"))
	} else if strings.HasPrefix(entry, "~/") {
		entry = filepath.Join(homeDir, strings.TrimPrefix(entry, "~/"))
	} else if entry == "$HOME" || entry == "${HOME}" || entry == "~" {
		entry = homeDir
	}
	return filepath.Clean(entry) == filepath.Clean(installDir)
}

func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func shellWords(value string) []string {
	var words []string
	var current strings.Builder
	var quote rune
	escaped := false
	flush := func() {
		if current.Len() > 0 {
			words = append(words, current.String())
			current.Reset()
		}
	}

	for _, char := range value {
		if escaped {
			current.WriteRune(char)
			escaped = false
			continue
		}
		if char == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote != 0 {
			if char == quote {
				quote = 0
			} else {
				current.WriteRune(char)
			}
			continue
		}
		switch char {
		case '\'', '"':
			quote = char
		case ' ', '\t', ';':
			flush()
		default:
			current.WriteRune(char)
		}
	}
	if escaped {
		current.WriteRune('\\')
	}
	flush()
	return words
}

func managedBlockRangeFor(contents, startMarker, endMarker string) (int, int, bool, error) {
	starts := strings.Count(contents, startMarker)
	ends := strings.Count(contents, endMarker)
	if starts == 0 && ends == 0 {
		return 0, 0, false, nil
	}
	if starts != 1 || ends != 1 {
		return 0, 0, false, fmt.Errorf("marcadores gerenciados inválidos; remova ou repare o bloco do Caramel manualmente")
	}
	start := strings.Index(contents, startMarker)
	endStart := strings.Index(contents[start:], endMarker)
	if endStart < 0 {
		return 0, 0, false, errors.New("marcador final ausente")
	}
	end := start + endStart + len(endMarker)
	if strings.HasPrefix(contents[end:], "\r\n") {
		end += 2
	} else if strings.HasPrefix(contents[end:], "\n") {
		end++
	}
	return start, end, true, nil
}

func installManagedBlockFor(contents, startMarker, endMarker, block string) (string, error) {
	start, end, found, err := managedBlockRangeFor(contents, startMarker, endMarker)
	if err != nil {
		return "", err
	}
	if found {
		return contents[:start] + block + contents[end:], nil
	}
	if contents == "" {
		return block, nil
	}
	return contents + lineEndingFor(contents) + block, nil
}

func removeManagedBlockFor(contents, startMarker, endMarker string) (string, error) {
	start, end, found, err := managedBlockRangeFor(contents, startMarker, endMarker)
	if err != nil || !found {
		return contents, err
	}
	if start >= 2 && contents[start-2:start] == "\r\n" {
		if start >= 4 && contents[start-4:start-2] == "\r\n" {
			start -= 2
		}
	} else if start >= 1 && contents[start-1:start] == "\n" {
		if start >= 2 && contents[start-2:start-1] == "\n" {
			start--
		}
	}
	return contents[:start] + contents[end:], nil
}
