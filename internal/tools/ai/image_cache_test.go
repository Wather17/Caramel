package ai_test

import (
	"testing"

	"caramel/internal/tools/ai"
)

func TestImageCacheKeyNormalizesConceptWhitespaceAndCase(t *testing.T) {
	base := ai.ImageCacheKey(ai.ImageCacheOptions{
		Concept: " Bolo   de fubá ", Style: "3d-cute", Aspect: "1:1", Model: "provider/model",
	})
	equivalent := ai.ImageCacheKey(ai.ImageCacheOptions{
		Concept: "bolo de fubá", Style: "3D-CUTE", Aspect: " 1:1 ", Model: "PROVIDER/MODEL",
	})
	if base != equivalent {
		t.Fatalf("valores equivalentes produziram chaves distintas: %s != %s", base, equivalent)
	}
}

func TestImageCacheKeySeparatesVisualSettingsAndContext(t *testing.T) {
	base := ai.ImageCacheKey(ai.ImageCacheOptions{
		Concept: "bolo", Theme: "padaria", Style: "clipart", Aspect: "1:1", Model: "provider/model",
	})
	variants := []ai.ImageCacheOptions{
		{Concept: "bolo", Theme: "festa", Style: "clipart", Aspect: "1:1", Model: "provider/model"},
		{Concept: "bolo", Theme: "padaria", Style: "realistic", Aspect: "1:1", Model: "provider/model"},
		{Concept: "bolo", Theme: "padaria", Style: "clipart", Aspect: "16:9", Model: "provider/model"},
		{Concept: "bolo", Theme: "padaria", Style: "clipart", Aspect: "1:1", Model: "provider/other"},
		{Concept: "bolo", Theme: "padaria", CustomStyle: "paper cutout", Aspect: "1:1", Model: "provider/model"},
	}
	for i, variant := range variants {
		if key := ai.ImageCacheKey(variant); key == base {
			t.Errorf("variante %d deveria produzir chave distinta", i)
		}
	}
}

func TestImageCacheKeyUsesCustomStyleAsEffectiveStyle(t *testing.T) {
	one := ai.ImageCacheKey(ai.ImageCacheOptions{
		Concept: "bolo", Style: "clipart", CustomStyle: "Soft clay illustration", Aspect: "1:1", Model: "provider/model",
	})
	two := ai.ImageCacheKey(ai.ImageCacheOptions{
		Concept: "bolo", Style: "realistic", CustomStyle: "soft clay illustration", Aspect: "1:1", Model: "provider/model",
	})
	if one != two {
		t.Fatalf("estilos predefinidos ignorados quando custom-style está ativo deveriam coincidir")
	}
}
