package cli

import (
	"strings"
	"testing"

	"caramel/internal/tools/ai"
)

func TestObfuscateKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want string
	}{
		{name: "chave longa mostra prefixo e sufixo", key: "sk-or-v1-abcdefgh1234", want: "sk-or-...1234"},
		{name: "chave curta vira asteriscos", key: "abc", want: "****"},
		{name: "chave com 8 chars vira asteriscos", key: "12345678", want: "****"},
		{name: "chave com 9 chars é ofuscada", key: "123456789", want: "123456...6789"},
		{name: "chave vazia vira asteriscos", key: "", want: "****"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := obfuscateKey(tt.key); got != tt.want {
				t.Errorf("obfuscateKey(%q) = %q, esperado %q", tt.key, got, tt.want)
			}
		})
	}
}

func TestDescribeModel(t *testing.T) {
	if got := describeModel("google/gemini-flash", "openai/gpt"); !strings.HasPrefix(got, "✅") || !strings.Contains(got, "(configurado)") {
		t.Errorf("modelo configurado deveria aparecer com ✅ e '(configurado)', obtido %q", got)
	}
	if got := describeModel("", "openai/gpt-x"); !strings.HasPrefix(got, "❌") || !strings.Contains(got, "openai/gpt-x") {
		t.Errorf("modelo sem config deveria mostrar ❌ e o padrão de fábrica, obtido %q", got)
	}
}

func TestOrDefault(t *testing.T) {
	if got := orDefault("valor", "fallback"); got != "valor" {
		t.Errorf("orDefault com valor preenchido = %q", got)
	}
	if got := orDefault("", "fallback"); got != "fallback" {
		t.Errorf("orDefault com valor vazio = %q, esperado fallback", got)
	}
}

func TestTruncateModelName(t *testing.T) {
	curto := "vendor/modelo-curto"
	if got := truncateModelName(curto, 45); got != curto {
		t.Errorf("nome dentro do limite não deveria mudar, obtido %q", got)
	}

	got := truncateModelName(strings.Repeat("a", 60), 45)
	runes := []rune(got)
	if len(runes) != 45 || !strings.HasSuffix(got, "…") {
		t.Errorf("nome longo deveria virar 45 runes terminando em '…', obtido %d runes terminando em %q", len(runes), got[len(got)-3:])
	}
}

func TestListModelsPlain(t *testing.T) {
	models := []ai.Model{
		{ID: "b/imagem-cara", Name: "Imagem Cara", PromptPrice: 2.0, Architecture: ai.Arch{OutputModalities: []string{"image"}}},
		{ID: "a/imagem-barata", Name: "Imagem Barata", PromptPrice: 0.1, Architecture: ai.Arch{OutputModalities: []string{"image"}}},
		{ID: "c/texto", Name: "Texto", PromptPrice: 0.05, Architecture: ai.Arch{InputModalities: []string{"text"}, OutputModalities: []string{"text"}}},
	}

	configModelsRole = ai.RoleImage
	configModelsQuery = ""
	configModelsLimit = 1
	defer func() { configModelsRole, configModelsQuery, configModelsLimit = "", "", 0 }()

	out := captureStdout(t, func() {
		if err := listModelsPlain(models); err != nil {
			t.Errorf("listModelsPlain falhou: %v", err)
		}
	})

	if !strings.Contains(out, "🎨 Modelos de Imagem") {
		t.Errorf("saída deveria ter título de modelos de imagem, obtido: %s", out)
	}
	if !strings.Contains(out, "exibindo 1") {
		t.Errorf("--limit 1 deveria limitar a exibição, obtido: %s", out)
	}
	idxCara := strings.Index(out, "b/imagem-cara")
	idxBarata := strings.Index(out, "a/imagem-barata")
	if idxBarata == -1 {
		t.Errorf("modelo mais barato deveria aparecer primeiro na ordenação por preço, obtido: %s", out)
	}
	if idxCara != -1 {
		t.Errorf("--limit 1 deveria ocultar o segundo modelo, obtido: %s", out)
	}
}
