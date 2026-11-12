package cli

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"
	"time"

	"caramel/internal/vault"
)

func TestUsageCatalogPrefersCanonicalPathsAndIncludesGlobalFlags(t *testing.T) {
	catalog := usageCatalog(RootCmd)
	hasCanonical := false
	hasLegacy := false
	for _, command := range catalog.Commands {
		if command == "caramel print 2up" {
			hasCanonical = true
		}
		if command == "caramel 2up" {
			hasLegacy = true
		}
	}
	if !hasCanonical || hasLegacy {
		t.Fatalf("catálogo deveria preferir o caminho canônico: %+v", catalog.Commands)
	}
	hasJSON := false
	for _, flag := range catalog.Flags {
		if flag.CommandPath == "caramel version" && flag.Name == "json" {
			hasJSON = true
			break
		}
	}
	if !hasJSON {
		t.Fatal("flags globais deveriam aparecer no catálogo do comando")
	}
}

func TestUsageOutputHumanAndCSVFormats(t *testing.T) {
	last := time.Date(2026, time.November, 12, 12, 0, 0, 0, time.UTC)
	report := vault.DetailedUsageReport{
		Total:    1,
		Coverage: vault.UsageCoverage{Events: 1, ActiveDays: 1, ObservedDays: 1, HasData: true, FirstCollectedAt: &last, LastCollectedAt: &last},
		Commands: []vault.UsageCommand{{Name: "caramel version", Count: 1, Share: 1, ActiveDays: 1, LastUsed: &last, DurationSamples: 1, AverageDurationMS: 2, Status: vault.UsageStatusCounts{Success: 1}}},
		Flags:    []vault.UsageFlag{{CommandPath: "caramel version", Name: "json", Count: 1, LastUsed: &last}},
		Trend:    []vault.UsageTrend{{Start: last, End: last.Add(time.Hour), Count: 1}},
		Signals:  []vault.UsageSignal{{Kind: "failed_command", CommandPath: "caramel version", Detail: "investigar"}},
	}

	var human bytes.Buffer
	if err := writeDetailedUsageHuman(&human, report, "últimos 30 dias"); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"Cobertura da evidência", "caramel version", "Tendência semanal", "Sinais para revisão humana"} {
		if !strings.Contains(human.String(), expected) {
			t.Errorf("saída humana deveria conter %q: %s", expected, human.String())
		}
	}

	var csvOutput bytes.Buffer
	if err := writeUsageCSV(&csvOutput, report, "últimos 30 dias"); err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(&csvOutput).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 3 || len(rows[0]) != 3 || rows[0][0] != "tipo" || rows[0][1] != "nome" || rows[0][2] != "contagem" {
		t.Fatalf("CSV incompatível com o contrato anterior: %#v", rows)
	}
}
