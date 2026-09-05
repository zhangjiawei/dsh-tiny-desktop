package core

import "testing"

func TestWindowsCloudReparsePolicyIsNarrow(t *testing.T) {
	const reparse = windowsFileAttributeReparsePoint
	for _, tag := range []uint32{
		windowsReparseCloud,
		windowsReparseCloud | 0x00001000,
		windowsReparseCloud | 0x0000F000,
		windowsReparseFilePlaceholder,
		windowsReparseOneDrive,
		windowsReparseStorageSync,
	} {
		if !windowsCloudReparseAllowed(reparse, tag) {
			t.Fatalf("known cloud data tag 0x%08X was rejected", tag)
		}
	}
	for _, tag := range []uint32{
		0xA0000003, // IO_REPARSE_TAG_MOUNT_POINT / junction
		0xA000000C, // IO_REPARSE_TAG_SYMLINK
		0x80000023, // IO_REPARSE_TAG_AF_UNIX
		0x9000001C, // IO_REPARSE_TAG_PROJFS
		0x8000001B, // IO_REPARSE_TAG_APPEXECLINK
	} {
		if windowsCloudReparseAllowed(reparse, tag) {
			t.Fatalf("unsafe or non-cloud tag 0x%08X was accepted", tag)
		}
	}
	if windowsCloudReparseAllowed(0, windowsReparseCloud) {
		t.Fatal("cloud tag without FILE_ATTRIBUTE_REPARSE_POINT was accepted")
	}
}
