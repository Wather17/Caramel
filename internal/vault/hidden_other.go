//go:build !windows

package vault

func isPlatformHidden(string) bool { return false }
