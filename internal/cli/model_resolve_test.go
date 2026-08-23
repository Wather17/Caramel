package cli

import "testing"

func TestResolveModel(t *testing.T) {
	tests := []struct {
		name        string
		flagVal     string
		flagChanged bool
		cfgVal      string
		want        string
	}{
		{
			name:    "flag explicita tem prioridade sobre config",
			flagVal: "anthropic/claude-sonnet-4.5", flagChanged: true, cfgVal: "google/x",
			want: "anthropic/claude-sonnet-4.5",
		},
		{
			name:    "config usado quando flag nao foi passada",
			flagVal: "google/gemini-3.1-flash-image-preview", flagChanged: false, cfgVal: "qwen/qwen3.7-flash",
			want: "qwen/qwen3.7-flash",
		},
		{
			name:    "default de fabrica quando nada configurado",
			flagVal: "google/gemini-3.1-flash-image-preview", flagChanged: false, cfgVal: "",
			want: "google/gemini-3.1-flash-image-preview",
		},
		{
			name:    "flag vazia explicita cai no config",
			flagVal: "", flagChanged: true, cfgVal: "deepseek/deepseek-v4-flash",
			want: "deepseek/deepseek-v4-flash",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveModel(tt.flagVal, tt.flagChanged, tt.cfgVal)
			if got != tt.want {
				t.Errorf("resolveModel() = %q, esperado %q", got, tt.want)
			}
		})
	}
}