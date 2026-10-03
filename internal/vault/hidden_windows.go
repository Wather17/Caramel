//go:build windows

package vault

import "golang.org/x/sys/windows"

func isPlatformHidden(path string) bool {
	pointer, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false
	}
	attributes, err := windows.GetFileAttributes(pointer)
	return err == nil && attributes&windows.FILE_ATTRIBUTE_HIDDEN != 0
}
