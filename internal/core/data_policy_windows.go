//go:build windows

package core

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

func inspectImportEntry(name string, info fs.FileInfo) (bool, string, error) {
	mode := info.Mode()
	if mode.IsRegular() || info.IsDir() && mode&fs.ModeIrregular == 0 {
		return true, "", nil
	}
	attributes, tag, err := windowsImportMetadata(name)
	if err != nil {
		return false, "", err
	}
	if windowsCloudReparseAllowed(attributes, tag) {
		return true, "", nil
	}
	if attributes&windowsFileAttributeReparsePoint != 0 {
		return false, fmt.Sprintf("Windows reparse tag=0x%08X, attributes=0x%08X", tag, attributes), nil
	}
	return false, importModeDetail(mode), nil
}

func windowsImportMetadata(name string) (uint32, uint32, error) {
	extended, err := windowsExtendedPath(name)
	if err != nil {
		return 0, 0, err
	}
	pointer, err := windows.UTF16PtrFromString(extended)
	if err != nil {
		return 0, 0, err
	}
	var data windows.Win32finddata
	handle, err := windows.FindFirstFile(pointer, &data)
	if err != nil {
		return 0, 0, err
	}
	defer windows.FindClose(handle)
	return data.FileAttributes, data.Reserved0, nil
}

func windowsExtendedPath(name string) (string, error) {
	absolute, err := filepath.Abs(name)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(absolute, `\\?\`) {
		return absolute, nil
	}
	if strings.HasPrefix(absolute, `\\`) {
		return `\\?\UNC\` + strings.TrimPrefix(absolute, `\\`), nil
	}
	return `\\?\` + absolute, nil
}
