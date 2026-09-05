//go:build !windows

package core

import "io/fs"

func inspectImportEntry(_ string, info fs.FileInfo) (bool, string, error) {
	if info.IsDir() || info.Mode().IsRegular() {
		return true, "", nil
	}
	return false, importModeDetail(info.Mode()), nil
}
