package compress

import (
	"fmt"
	"strconv"
	"strings"
)

type Preset string

const (
	PresetLight    Preset = "light"
	PresetBalanced Preset = "balanced"
	PresetHigh     Preset = "high"
)

type Options struct {
	Preset     Preset
	Quality    int
	MaxDim     int
	KeepFormat bool
	MaxDimSet  bool
}

type ResolvedOptions struct {
	Quality, MaxDim int
	KeepFormat      bool
}

func ResolveOptions(opts Options) (ResolvedOptions, error) {
	preset := opts.Preset
	if preset == "" {
		preset = PresetBalanced
	}
	quality, maxDim := 80, 1920
	switch preset {
	case PresetLight:
		quality, maxDim = 65, 1280
	case PresetBalanced:
	case PresetHigh:
		quality, maxDim = 90, 2560
	default:
		return ResolvedOptions{}, fmt.Errorf("preset desconhecido %q", preset)
	}
	if opts.Quality != 0 {
		quality = opts.Quality
	}
	if opts.MaxDimSet || opts.MaxDim != 0 {
		maxDim = opts.MaxDim
	}
	if quality < 1 || quality > 100 {
		return ResolvedOptions{}, fmt.Errorf("quality deve estar entre 1 e 100")
	}
	if maxDim < 0 {
		return ResolvedOptions{}, fmt.Errorf("max-dim não pode ser negativo")
	}
	return ResolvedOptions{Quality: quality, MaxDim: maxDim, KeepFormat: opts.KeepFormat}, nil
}

func ParseSizeInBytes(s string) (int64, error) {
	v := strings.TrimSpace(strings.ToUpper(s))
	if v == "" {
		return 0, nil
	}
	m := int64(1)
	for _, suffix := range []struct {
		s string
		m int64
	}{{"KB", 1024}, {"MB", 1024 * 1024}, {"B", 1}, {"K", 1024}} {
		if strings.HasSuffix(v, suffix.s) {
			m = suffix.m
			v = strings.TrimSpace(strings.TrimSuffix(v, suffix.s))
			break
		}
	}
	n, e := strconv.ParseInt(v, 10, 64)
	if e != nil || n < 0 {
		return 0, fmt.Errorf("tamanho inválido %q", s)
	}
	return n * m, nil
}
