package ui

import "testing"

func TestGetCaramelTheme(t *testing.T) {
	theme := GetCaramelTheme()
	if theme == nil {
		t.Fatal("GetCaramelTheme deveria retornar um tema não nulo")
	}
	if again := GetCaramelTheme(); again == nil {
		t.Error("segunda chamada deveria continuar retornando um tema válido")
	}
}
