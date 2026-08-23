package docx

import "testing"

func TestIsColorableFormat(t *testing.T) {
	colorable := []string{"png", "PNG", "jpg", "jpeg", "webp"}
	for _, f := range colorable {
		if !IsColorableFormat(f) {
			t.Errorf("formato '%s' deveria ser colorível", f)
		}
	}

	notColorable := []string{"emf", "wmf", "bin", "svg", "tiff", "gif", "bmp", "", "jpeg2000"}
	for _, f := range notColorable {
		if IsColorableFormat(f) {
			t.Errorf("formato '%s' não deveria ser colorível", f)
		}
	}
}

func TestParseSizeInBytes(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		{name: "kb", input: "20KB", want: 20 * 1024},
		{name: "mb", input: "1MB", want: 1024 * 1024},
		{name: "bytes explicito", input: "500B", want: 500},
		{name: "numero puro em bytes", input: "20", want: 20},
		{name: "k minúsculo com espaços", input: "  10 kb ", want: 10 * 1024},
		{name: "sufixo k simples", input: "2k", want: 2 * 1024},
		{name: "vazio retorna zero sem erro", input: "", want: 0},
		{name: "apenas espaços retorna zero", input: "   ", want: 0},
		{name: "texto invalido", input: "abc", wantErr: true},
		{name: "unidade desconhecida", input: "10GB", wantErr: true},
		{name: "negativo é rejeitado pelo chamador, mas parse aceita sinal", input: "-1KB", want: -1024},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSizeInBytes(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseSizeInBytes(%q) err = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ParseSizeInBytes(%q) = %d, esperado %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestFilterImagesByMinSize(t *testing.T) {
	images := []ExtractedImage{
		{OriginalName: "grande.png", Size: 500_000},
		{OriginalName: "limite.png", Size: 1024},
		{OriginalName: "pequena.png", Size: 100},
	}

	kept, skipped := FilterImagesByMinSize(images, 1024)

	if len(kept) != 2 || kept[0].OriginalName != "grande.png" || kept[1].OriginalName != "limite.png" {
		t.Errorf("esperado [grande limite] mantidas (>= inclui a igual ao limite), obtido %+v", kept)
	}
	if len(skipped) != 1 || skipped[0].OriginalName != "pequena.png" {
		t.Errorf("esperado apenas 'pequena' pulada, obtido %+v", skipped)
	}
}

func TestFilterImagesByNames(t *testing.T) {
	images := []ExtractedImage{
		{OriginalName: "image1.png"},
		{OriginalName: "image2.png"},
		{OriginalName: "image3.webp"},
	}

	got := FilterImagesByNames(images, []string{"image3.webp", "image1.png"})

	want := []ExtractedImage{{OriginalName: "image1.png"}, {OriginalName: "image3.webp"}}
	if len(got) != len(want) {
		t.Fatalf("esperado %d imagens na ordem [image1 image3], obtido %+v", len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("posição %d: esperado %+v, obtido %+v (ordem deve seguir a lista de nomes)", i, want[i], got[i])
		}
	}
	if len(FilterImagesByNames(images, nil)) != 0 {
		t.Error("lista de nomes vazia deveria retornar nenhuma imagem")
	}
}