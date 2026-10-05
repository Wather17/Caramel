package ui

import (
	"reflect"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestFilePickerModelConfirmsSelectedFilesInDisplayOrder(t *testing.T) {
	model := NewFilePickerModel("Selecionar", []FilePickerItem{
		{Path: "a.docx", Name: "A"},
		{Path: "b.docx", Name: "B"},
		{Path: "c.docx", Name: "C"},
	}, 2)

	model = updateFilePickerModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	model = updateFilePickerModel(t, model, tea.KeyMsg{Type: tea.KeyDown})
	model = updateFilePickerModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	updated, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(FilePickerModel)
	if command == nil {
		t.Fatal("Enter deveria encerrar após atingir o mínimo de seleções")
	}
	result := model.Result()
	if result.Action != FilePickerConfirmed {
		t.Fatalf("ação inesperada: %q", result.Action)
	}
	if want := []string{"a.docx", "b.docx"}; !reflect.DeepEqual(result.Selected, want) {
		t.Fatalf("seleção fora da ordem exibida: want=%v got=%v", want, result.Selected)
	}
}

func TestFilePickerModelRequiresMinimumBeforeConfirming(t *testing.T) {
	model := NewFilePickerModel("Selecionar", []FilePickerItem{
		{Path: "a.docx", Name: "A"},
		{Path: "b.docx", Name: "B"},
	}, 2)
	model = updateFilePickerModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	updated, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(FilePickerModel)
	if command != nil {
		t.Fatal("Enter não deveria encerrar com uma única seleção")
	}
	if model.status == "" {
		t.Fatal("o seletor deveria informar o mínimo de seleções")
	}
}

func TestFilePickerModelSearchesNameAndIgnoresAccents(t *testing.T) {
	model := NewFilePickerModel("Selecionar", []FilePickerItem{
		{Path: "C:/docs/Relatório final.docx", Name: "Relatório final.docx"},
		{Path: "C:/docs/atividade.docx", Name: "Atividade.docx"},
	}, 1)
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("relatorio")})
	model = updated.(FilePickerModel)
	if got := len(model.filtered); got != 1 {
		t.Fatalf("busca deveria encontrar um DOCX sem considerar acento, encontrou %d", got)
	}
	if model.items[model.filtered[0]].Path != "C:/docs/Relatório final.docx" {
		t.Fatalf("resultado inesperado: %v", model.items[model.filtered[0]])
	}
}

func TestFilePickerModelCtrlOReturnsHighlightedFile(t *testing.T) {
	model := NewFilePickerModel("Selecionar", []FilePickerItem{
		{Path: "a.docx", Name: "A"},
		{Path: "b.docx", Name: "B"},
	}, 2)
	model = updateFilePickerModel(t, model, tea.KeyMsg{Type: tea.KeyDown})
	updated, command := model.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	model = updated.(FilePickerModel)
	if command == nil {
		t.Fatal("Ctrl+O deveria encerrar a TUI")
	}
	result := model.Result()
	if result.Action != FilePickerReveal || result.RevealPath != "b.docx" {
		t.Fatalf("resultado de Ctrl+O inesperado: %+v", result)
	}
}

func TestFilePickerModelEscapeCancels(t *testing.T) {
	model := NewFilePickerModel("Selecionar", []FilePickerItem{{Path: "a.docx", Name: "A"}}, 1)
	updated, command := model.Update(tea.KeyMsg{Type: tea.KeyEscape})
	model = updated.(FilePickerModel)
	if command == nil || model.Result().Action != FilePickerCanceled {
		t.Fatalf("Esc deveria cancelar o seletor: action=%q command=%v", model.Result().Action, command)
	}
}

func updateFilePickerModel(t *testing.T, model FilePickerModel, msg tea.Msg) FilePickerModel {
	t.Helper()
	updated, _ := model.Update(msg)
	return updated.(FilePickerModel)
}
