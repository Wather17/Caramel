package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"caramel/internal/tools/ai"
	"caramel/internal/vault"
)

var errMissingOpenRouterKey = errors.New("chave de API do OpenRouter não configurada. Use 'caramel config setup' ou 'caramel config set openrouter_key <sua-chave>'")

type imageGenerationGroup struct {
	key     string
	concept string
	indexes []int
	item    ai.GenerationItem
	cached  *vault.GeneratedImage
}

func executeImageGeneration(
	ctx context.Context,
	rawItems []string,
	theme string,
	cfg ai.HarnessConfig,
	targetDir string,
	cache *vault.Vault,
	refreshCache bool,
	client *ai.Client,
	onProgress ai.HarnessProgressFunc,
) ([]ai.GenerationItem, []string, error) {
	if cache == nil {
		if client == nil {
			return nil, nil, errMissingOpenRouterKey
		}
		items, err := ai.SynthesizePromptsContext(ctx, cfg, client)
		if err != nil {
			return nil, nil, err
		}
		cfg.OutputDir = targetDir
		results, err := ai.ExecuteGenerationHarnessContext(ctx, items, cfg, client, onProgress)
		return results, nil, err
	}

	themeMode := len(rawItems) == 0 && strings.TrimSpace(theme) != ""
	var warnings []string
	var groups []*imageGenerationGroup
	groupByKey := make(map[string]*imageGenerationGroup)
	var themeItems []ai.GenerationItem

	if themeMode {
		if client == nil {
			return nil, warnings, errMissingOpenRouterKey
		}
		cfg.Items = nil
		cfg.Theme = theme
		var err error
		themeItems, err = ai.SynthesizePromptsContext(ctx, cfg, client)
		if err != nil {
			return nil, warnings, err
		}
		for i := range themeItems {
			concept := themeItems[i].Name
			key := imageCacheKey(concept, theme, cfg)
			group := groupByKey[key]
			if group == nil {
				group = &imageGenerationGroup{key: key, concept: concept, item: themeItems[i]}
				groupByKey[key] = group
				groups = append(groups, group)
			}
			group.indexes = append(group.indexes, i)
		}
	} else {
		for i, concept := range rawItems {
			key := imageCacheKey(concept, "", cfg)
			group := groupByKey[key]
			if group == nil {
				group = &imageGenerationGroup{key: key, concept: concept}
				groupByKey[key] = group
				groups = append(groups, group)
			}
			group.indexes = append(group.indexes, i)
		}
	}

	if !refreshCache {
		for _, group := range groups {
			entry, found, err := cache.LookupGeneratedImage(ctx, group.key)
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return nil, warnings, err
				}
				warnings = append(warnings, fmt.Sprintf("não foi possível consultar a biblioteca para %s; o Caramel seguirá sem reutilizar essa imagem", group.concept))
				continue
			}
			if found {
				group.cached = &entry
				group.item = ai.GenerationItem{
					Name:   entry.Name,
					Prompt: entry.Prompt,
					Format: entry.Extension,
					Status: "done",
				}
			}
		}
	}

	total := 0
	for _, group := range groups {
		total += len(group.indexes)
	}
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return nil, warnings, fmt.Errorf("falha ao criar pasta de destino '%s': %w", targetDir, err)
	}
	results := make([]ai.GenerationItem, total)
	cacheCompleted := 0
	for _, group := range groups {
		if group.cached == nil {
			continue
		}
		copyFailed := false
		groupResults := make([]ai.GenerationItem, 0, len(group.indexes))
		for _, originalIndex := range group.indexes {
			item := group.item
			item.Index = originalIndex + 1
			item.Slug = ai.SanitizeSlug(item.Index, item.Name)
			destination := filepath.Join(targetDir, item.Slug+"."+group.cached.Extension)
			if err := copyImageFile(group.cached.Path, destination); err != nil {
				warnings = append(warnings, fmt.Sprintf("não foi possível copiar %s da biblioteca; o Caramel tentará gerar a imagem", group.concept))
				copyFailed = true
				break
			}
			item.Status = "done"
			item.ImagePath = destination
			item.Format = group.cached.Extension
			item.Reused = true
			groupResults = append(groupResults, item)
		}
		if copyFailed {
			group.cached = nil
			continue
		}
		for i, originalIndex := range group.indexes {
			results[originalIndex] = groupResults[i]
			cacheCompleted++
			if onProgress != nil {
				onProgress(ai.HarnessProgressEvent{Item: groupResults[i], Total: total, Completed: cacheCompleted, Success: cacheCompleted, CurrentStep: "saved"})
			}
		}
	}

	if !themeMode {
		var conceptsToSynthesize []string
		var synthGroups []*imageGenerationGroup
		for _, group := range groups {
			if group.cached == nil && group.item.Prompt == "" {
				conceptsToSynthesize = append(conceptsToSynthesize, group.concept)
				synthGroups = append(synthGroups, group)
			}
		}
		if len(conceptsToSynthesize) > 0 {
			if client == nil {
				return results, warnings, errMissingOpenRouterKey
			}
			synthCfg := cfg
			synthCfg.Items = conceptsToSynthesize
			synthCfg.Theme = ""
			items, err := ai.SynthesizePromptsContext(ctx, synthCfg, client)
			if err != nil {
				return results, warnings, err
			}
			if len(items) != len(synthGroups) {
				return results, warnings, fmt.Errorf("a síntese retornou %d item(ns) para %d conceito(s); não é seguro associar os prompts à biblioteca", len(items), len(synthGroups))
			}
			for i, item := range items {
				group := synthGroups[i]
				group.item = item
			}
		}
	}

	var missingGroups []*imageGenerationGroup
	var missingItems []ai.GenerationItem
	for _, group := range groups {
		if group.cached != nil {
			continue
		}
		if group.item.Prompt == "" {
			return results, warnings, fmt.Errorf("não há prompt disponível para gerar %s", group.concept)
		}
		firstIndex := group.indexes[0]
		group.item.Index = firstIndex + 1
		group.item.Slug = ai.SanitizeSlug(group.item.Index, group.item.Name)
		missingGroups = append(missingGroups, group)
		missingItems = append(missingItems, group.item)
	}
	if len(missingItems) == 0 {
		return results, warnings, nil
	}
	if client == nil {
		return results, warnings, errMissingOpenRouterKey
	}

	cfg.OutputDir = targetDir
	generated, err := ai.ExecuteGenerationHarnessContext(ctx, missingItems, cfg, client, func(event ai.HarnessProgressEvent) {
		if onProgress == nil {
			return
		}
		event.Total = total
		event.Completed += cacheCompleted
		event.Success += cacheCompleted
		if event.Item.Status == "error" {
			event.CurrentStep = "error"
		}
		onProgress(event)
	})
	if err != nil {
		return results, warnings, err
	}
	if len(generated) != len(missingGroups) {
		return results, warnings, fmt.Errorf("o harness retornou %d resultado(s) para %d item(ns)", len(generated), len(missingGroups))
	}

	for i, generatedItem := range generated {
		group := missingGroups[i]
		if generatedItem.Status == "done" && generatedItem.ImagePath != "" {
			imageBytes, readErr := os.ReadFile(generatedItem.ImagePath)
			if readErr != nil {
				warnings = append(warnings, fmt.Sprintf("não foi possível salvar %s na biblioteca de imagens", group.concept))
			} else if _, storeErr := cache.StoreGeneratedImage(ctx, group.key, generatedItem.Name, generatedItem.Prompt, generatedItem.Format, imageBytes); storeErr != nil {
				warnings = append(warnings, fmt.Sprintf("não foi possível salvar %s na biblioteca de imagens", group.concept))
			}
		}
		for position, originalIndex := range group.indexes {
			item := generatedItem
			item.Index = originalIndex + 1
			item.Slug = ai.SanitizeSlug(item.Index, item.Name)
			if position > 0 && item.Status == "done" && item.ImagePath != "" {
				destination := filepath.Join(targetDir, item.Slug+"."+item.Format)
				if err := copyImageFile(generatedItem.ImagePath, destination); err != nil {
					item.Status = "error"
					item.ImagePath = ""
					item.Error = fmt.Sprintf("falha ao salvar arquivo duplicado: %v", err)
				} else {
					item.ImagePath = destination
					item.Reused = true
					if onProgress != nil {
						onProgress(ai.HarnessProgressEvent{Item: item, Total: total, Completed: originalIndex + 1, CurrentStep: "saved"})
					}
				}
			}
			results[originalIndex] = item
		}
	}
	return results, warnings, nil
}

func imageCacheKey(concept, theme string, cfg ai.HarnessConfig) string {
	return ai.ImageCacheKey(ai.ImageCacheOptions{
		Concept:     concept,
		Theme:       theme,
		Style:       cfg.Style,
		CustomStyle: cfg.CustomStyle,
		Aspect:      cfg.Aspect,
		Model:       cfg.ImageModel,
	})
}

func copyImageFile(source, destination string) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		return err
	}
	return os.WriteFile(destination, data, 0644)
}
