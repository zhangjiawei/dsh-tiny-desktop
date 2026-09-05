package core

import "io/fs"

const (
	windowsFileAttributeReparsePoint = uint32(0x00000400)
	windowsReparseNameSurrogate      = uint32(0x20000000)
	windowsReparseCloud              = uint32(0x9000001A)
	windowsReparseCloudMask          = uint32(0x0000F000)
	windowsReparseFilePlaceholder    = uint32(0x80000015)
	windowsReparseOneDrive           = uint32(0x80000021)
	windowsReparseStorageSync        = uint32(0x8000002A)
)

// windowsCloudReparseAllowed is intentionally an allowlist, not a generic
// "all reparse points are files" exception. Cloud Files placeholders are
// provider-backed data objects that become readable through the filesystem;
// name-surrogate links, sockets, projected trees and unknown filter objects
// retain the fail-closed import policy.
func windowsCloudReparseAllowed(attributes, tag uint32) bool {
	if attributes&windowsFileAttributeReparsePoint == 0 || tag&windowsReparseNameSurrogate != 0 {
		return false
	}
	if tag&^windowsReparseCloudMask == windowsReparseCloud {
		return true
	}
	switch tag {
	case windowsReparseFilePlaceholder, windowsReparseOneDrive, windowsReparseStorageSync:
		return true
	default:
		return false
	}
}

func importModeDetail(mode fs.FileMode) string {
	switch {
	case mode&fs.ModeSocket != 0:
		return "socket"
	case mode&fs.ModeNamedPipe != 0:
		return "named pipe"
	case mode&fs.ModeDevice != 0:
		return "device"
	case mode&fs.ModeIrregular != 0:
		return "irregular file"
	default:
		return "mode=" + mode.String()
	}
}
