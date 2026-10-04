// Package platform integra o Caramel com recursos graficos do sistema operacional.
package platform

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Runner executa um programa diretamente, sem compor comandos de shell.
type Runner interface {
	LookPath(name string) (string, error)
	Run(name string, args ...string) error
}

// Opener abre resultados no aplicativo padrao ou no gerenciador de arquivos.
type Opener interface {
	Open(path string, bundle bool) error
	Reveal(path string, bundle bool) error
}

type systemRunner struct{}

func (systemRunner) LookPath(name string) (string, error) { return exec.LookPath(name) }
func (systemRunner) Run(name string, args ...string) error {
	command := exec.Command(name, args...)
	if err := command.Start(); err != nil {
		return err
	}
	return command.Process.Release()
}

type opener struct {
	goos   string
	runner Runner
}

// NewOpener cria o adaptador nativo da plataforma atual.
func NewOpener() Opener {
	return NewOpenerFor(runtime.GOOS, systemRunner{})
}

// NewOpenerFor permite testar a selecao de comandos sem iniciar aplicativos reais.
func NewOpenerFor(goos string, runner Runner) Opener {
	return &opener{goos: goos, runner: runner}
}

func (o *opener) Open(path string, bundle bool) error {
	switch o.goos {
	case "windows":
		if bundle {
			return o.run("explorer.exe", path)
		}
		return o.run("rundll32.exe", "url.dll,FileProtocolHandler", path)
	case "darwin":
		return o.run("open", path)
	case "linux":
		return o.run("xdg-open", path)
	default:
		return fmt.Errorf("abrir resultados não é suportado em %s", o.goos)
	}
}

func (o *opener) Reveal(path string, bundle bool) error {
	switch o.goos {
	case "windows":
		if bundle {
			return o.run("explorer.exe", path)
		}
		return o.run("explorer.exe", "/select,", path)
	case "darwin":
		if bundle {
			return o.run("open", path)
		}
		return o.run("open", "-R", path)
	case "linux":
		if !bundle {
			path = filepath.Dir(path)
		}
		return o.run("xdg-open", path)
	default:
		return fmt.Errorf("revelar resultados não é suportado em %s", o.goos)
	}
}

func (o *opener) run(name string, args ...string) error {
	resolved, err := o.runner.LookPath(name)
	if err != nil {
		return fmt.Errorf("aplicativo do sistema %q não está disponível: %w", name, err)
	}
	if err := o.runner.Run(resolved, args...); err != nil {
		return fmt.Errorf("não foi possível iniciar %q: %w", name, err)
	}
	return nil
}
