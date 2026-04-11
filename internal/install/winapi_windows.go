package install

import (
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modkernel32 = windows.NewLazySystemDLL("kernel32.dll")
	modadvapi32 = windows.NewLazySystemDLL("advapi32.dll")
	moduser32   = windows.NewLazySystemDLL("user32.dll")

	procCreateFileW         = modkernel32.NewProc("CreateFileW")
	procDeviceIoControl     = modkernel32.NewProc("DeviceIoControl")
	procSetFileAttributesW  = modkernel32.NewProc("SetFileAttributesW")
	procGetFileAttributesW  = modkernel32.NewProc("GetFileAttributesW")
	procRegOpenKeyExW       = modadvapi32.NewProc("RegOpenKeyExW")
	procRegQueryValueExW    = modadvapi32.NewProc("RegQueryValueExW")
	procRegSetValueExW      = modadvapi32.NewProc("RegSetValueExW")
	procRegCloseKey         = modadvapi32.NewProc("RegCloseKey")
	procRegDeleteValueW     = modadvapi32.NewProc("RegDeleteValueW")
	procSendMessageTimeoutW = moduser32.NewProc("SendMessageTimeoutW")
)

const (
	_HKEY_CURRENT_MACHINE uintptr = 0x80000002
	_HKEY_CURRENT_USER    uintptr = 0x80000001

	_KEY_READ      = 0x20019
	_KEY_WRITE     = 0x20006
	_REG_EXPAND_SZ = 2
	_REG_SZ        = 1

	_FILE_ATTRIBUTE_READONLY = 0x00000001

	_FSCTL_SET_REPARSE_POINT    = 0x000900A4
	_FSCTL_DELETE_REPARSE_POINT = 0x000900AC

	_IO_REPARSE_TAG_MOUNT_POINT = 0xA0000003

	_SMTO_ABORTIFHUNG = 0x0002
	_WM_SETTINGCHANGE = 0x001A

	_FILE_FLAG_OPEN_REPARSE_POINT = 0x00200000
	_FILE_FLAG_BACKUP_SEMANTICS   = 0x02000000

	_GENERIC_READ     = 0x80000000
	_GENERIC_WRITE    = 0x40000000
	_OPEN_EXISTING    = 3
	_FILE_SHARE_READ  = 0x00000001
	_FILE_SHARE_WRITE = 0x00000002

	_INVALID_HANDLE_VALUE = ^uintptr(0)
)

func nativeCreateJunction(link, target string) error {
	// Create the link directory first (required for DeviceIoControl SET_REPARSE_POINT).
	if err := os.MkdirAll(link, 0o755); err != nil {
		return fmt.Errorf("failed to create junction directory %s: %w", link, err)
	}

	linkPtr, err := windows.UTF16PtrFromString(link)
	if err != nil {
		return fmt.Errorf("failed to convert link path: %w", err)
	}

	handle, _, _ := procCreateFileW.Call(
		uintptr(unsafe.Pointer(linkPtr)),
		_GENERIC_READ|_GENERIC_WRITE,
		uintptr(_FILE_SHARE_READ|_FILE_SHARE_WRITE),
		0,
		_OPEN_EXISTING,
		uintptr(_FILE_FLAG_OPEN_REPARSE_POINT|_FILE_FLAG_BACKUP_SEMANTICS),
		0,
	)
	if handle == _INVALID_HANDLE_VALUE {
		return fmt.Errorf("CreateFileW failed for junction directory")
	}
	defer func() { _ = windows.CloseHandle(windows.Handle(handle)) }()

	// Junction reparse point data format:
	// The SubstituteName (printName) is the NT path like "\??\C:\path\to\target"
	// The PrintName is the Win32 path for display
	substituteName := `\??\` + target
	printName := target

	substituteUTF16 := windows.StringToUTF16(substituteName)
	printNameUTF16 := windows.StringToUTF16(printName)

	substituteNameLength := uint16(len(substituteUTF16)-1) * 2 // bytes, excluding null
	printNameLength := uint16(len(printNameUTF16)-1) * 2       // bytes, excluding null

	// REPARSE_MOUNTPOINT_DATA_BUFFER layout:
	//   ReparseTag (4) + ReparseDataLength (2) + Reserved (2) = 8 bytes header
	//   SubstituteNameOffset (2) + SubstituteNameLength (2) + PrintNameOffset (2) + PrintNameLength (2) = 8 bytes
	//   PathBuffer (variable)
	reparseDataLength := uint16(8 + substituteNameLength + 2 + printNameLength + 2)

	// Allocate buffer: header (8) + reparse data + path buffer (substitute + null + print + null)
	bufSize := 8 + int(reparseDataLength)
	buf := make([]byte, bufSize)

	// Write the REPARSE_MOUNTPOINT_DATA_BUFFER header
	*(*uint32)(unsafe.Pointer(&buf[0])) = _IO_REPARSE_TAG_MOUNT_POINT
	*(*uint16)(unsafe.Pointer(&buf[4])) = reparseDataLength
	*(*uint16)(unsafe.Pointer(&buf[6])) = 0 // Reserved

	// PathBuffer starts at offset 8
	pathBufOffset := 8
	*(*uint16)(unsafe.Pointer(&buf[pathBufOffset+0])) = 0                        // SubstituteNameOffset
	*(*uint16)(unsafe.Pointer(&buf[pathBufOffset+2])) = substituteNameLength     // SubstituteNameLength
	*(*uint16)(unsafe.Pointer(&buf[pathBufOffset+4])) = substituteNameLength + 2 // PrintNameOffset (after substitute + null)
	*(*uint16)(unsafe.Pointer(&buf[pathBufOffset+6])) = printNameLength          // PrintNameLength

	// Write substitute name at offset 16
	dataStart := pathBufOffset + 8
	copy(buf[dataStart:], unsafe.Slice((*byte)(unsafe.Pointer(&substituteUTF16[0])), substituteNameLength+2))

	// Write print name after substitute
	printOffset := dataStart + int(substituteNameLength) + 2
	copy(buf[printOffset:], unsafe.Slice((*byte)(unsafe.Pointer(&printNameUTF16[0])), printNameLength+2))

	var bytesReturned uint32
	ret, _, err := procDeviceIoControl.Call(
		handle,
		uintptr(_FSCTL_SET_REPARSE_POINT),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(bufSize),
		0,
		0,
		uintptr(unsafe.Pointer(&bytesReturned)),
		0,
	)
	if ret == 0 {
		// If setting reparse point fails, remove the empty directory we created.
		_ = os.Remove(link)
		return fmt.Errorf("DeviceIoControl FSCTL_SET_REPARSE_POINT failed: %w", err)
	}

	return nil
}

func nativeGetFileAttributes(path string) (uint32, error) {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	ret, _, _ := procGetFileAttributesW.Call(uintptr(unsafe.Pointer(pathPtr)))
	if ret == 0xFFFFFFFF {
		return 0, fmt.Errorf("GetFileAttributesW failed for %s", path)
	}
	return uint32(ret), nil
}

func nativeSetFileAttributes(path string, attrs uint32) error {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	ret, _, err := procSetFileAttributesW.Call(uintptr(unsafe.Pointer(pathPtr)), uintptr(attrs))
	if ret == 0 {
		return fmt.Errorf("SetFileAttributesW failed: %w", err)
	}
	return nil
}

func nativeRemoveReadOnly(path string) {
	attrs, err := nativeGetFileAttributes(path)
	if err != nil {
		return
	}
	_ = nativeSetFileAttributes(path, attrs & ^uint32(_FILE_ATTRIBUTE_READONLY))
}

func nativeSetReadOnly(path string) {
	attrs, err := nativeGetFileAttributes(path)
	if err != nil {
		return
	}
	_ = nativeSetFileAttributes(path, attrs|uint32(_FILE_ATTRIBUTE_READONLY))
}

func nativeRegOpenKey(hKey uintptr, subKey string, access uint32) (windows.Handle, error) {
	subKeyPtr, err := windows.UTF16PtrFromString(subKey)
	if err != nil {
		return 0, err
	}
	var handle windows.Handle
	ret, _, err := procRegOpenKeyExW.Call(
		hKey,
		uintptr(unsafe.Pointer(subKeyPtr)),
		0,
		uintptr(access),
		uintptr(unsafe.Pointer(&handle)),
	)
	if ret != 0 {
		return 0, fmt.Errorf("RegOpenKeyExW failed: %w", err)
	}
	return handle, nil
}

func nativeRegQueryValueEx(handle windows.Handle, name string) (string, uint32, error) {
	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return "", 0, err
	}

	var dataType uint32
	var dataSize uint32

	ret, _, err := procRegQueryValueExW.Call(
		uintptr(handle),
		uintptr(unsafe.Pointer(namePtr)),
		0,
		uintptr(unsafe.Pointer(&dataType)),
		0,
		uintptr(unsafe.Pointer(&dataSize)),
	)
	if ret != 0 && ret != 234 {
		return "", 0, fmt.Errorf("RegQueryValueExW (size query) failed: %w", err)
	}

	dataBuf := make([]byte, dataSize)
	ret, _, err = procRegQueryValueExW.Call(
		uintptr(handle),
		uintptr(unsafe.Pointer(namePtr)),
		0,
		uintptr(unsafe.Pointer(&dataType)),
		uintptr(unsafe.Pointer(&dataBuf[0])),
		uintptr(unsafe.Pointer(&dataSize)),
	)
	if ret != 0 {
		return "", 0, fmt.Errorf("RegQueryValueExW failed: %w", err)
	}

	if dataType == _REG_SZ || dataType == _REG_EXPAND_SZ {
		if dataSize >= 2 {
			utf16data := (*[1 << 28]uint16)(unsafe.Pointer(&dataBuf[0]))[:dataSize/2]
			s := windows.UTF16ToString(utf16data)
			return s, dataType, nil
		}
		return "", dataType, nil
	}

	return "", dataType, fmt.Errorf("unexpected registry type %d", dataType)
}

func nativeRegSetValueEx(handle windows.Handle, name string, dataType uint32, value string) error {
	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return err
	}

	utf16Value := windows.StringToUTF16(value)
	dataSize := uint32(len(utf16Value) * 2)

	ret, _, err := procRegSetValueExW.Call(
		uintptr(handle),
		uintptr(unsafe.Pointer(namePtr)),
		0,
		uintptr(dataType),
		uintptr(unsafe.Pointer(&utf16Value[0])),
		uintptr(dataSize),
	)
	if ret != 0 {
		return fmt.Errorf("RegSetValueExW failed: %w", err)
	}
	return nil
}

func nativeRegCloseKey(handle windows.Handle) {
	_, _, _ = procRegCloseKey.Call(uintptr(handle))
}

func nativeGetRegistryPath(scope string) (string, error) {
	var hKey uintptr
	subKey := "Environment"
	access := uint32(_KEY_READ)

	if scope == scoopScopeGlobal {
		hKey = _HKEY_CURRENT_MACHINE
		subKey = `SYSTEM\CurrentControlSet\Control\Session Manager\Environment`
	} else {
		hKey = _HKEY_CURRENT_USER
	}

	handle, err := nativeRegOpenKey(hKey, subKey, access)
	if err != nil {
		return "", err
	}
	defer nativeRegCloseKey(handle)

	value, _, err := nativeRegQueryValueEx(handle, "PATH")
	if err != nil {
		return "", err
	}
	return value, nil
}

func nativeSetRegistryPath(pathValue string, scope string) error {
	var hKey uintptr
	subKey := "Environment"
	access := uint32(_KEY_READ | _KEY_WRITE)

	if scope == scoopScopeGlobal {
		hKey = _HKEY_CURRENT_MACHINE
		subKey = `SYSTEM\CurrentControlSet\Control\Session Manager\Environment`
	} else {
		hKey = _HKEY_CURRENT_USER
	}

	handle, err := nativeRegOpenKey(hKey, subKey, access)
	if err != nil {
		return fmt.Errorf("failed to open registry key: %w", err)
	}
	defer nativeRegCloseKey(handle)

	return nativeRegSetValueEx(handle, "PATH", _REG_EXPAND_SZ, pathValue)
}

func nativeSetRegistryEnvVar(key, value, scope string) error {
	var hKey uintptr
	subKey := "Environment"
	access := uint32(_KEY_READ | _KEY_WRITE)

	if scope == scoopScopeGlobal {
		hKey = _HKEY_CURRENT_MACHINE
		subKey = `SYSTEM\CurrentControlSet\Control\Session Manager\Environment`
	} else {
		hKey = _HKEY_CURRENT_USER
	}

	handle, err := nativeRegOpenKey(hKey, subKey, access)
	if err != nil {
		return fmt.Errorf("failed to open registry key: %w", err)
	}
	defer nativeRegCloseKey(handle)

	if value == "" {
		return nativeRegDeleteValue(handle, key)
	}

	return nativeRegSetValueEx(handle, key, _REG_EXPAND_SZ, value)
}

func nativeRegDeleteValue(handle windows.Handle, name string) error {
	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	ret, _, err := procRegDeleteValueW.Call(
		uintptr(handle),
		uintptr(unsafe.Pointer(namePtr)),
	)
	if ret != 0 {
		if ret == uintptr(windows.ERROR_FILE_NOT_FOUND) {
			return nil
		}
		return fmt.Errorf("RegDeleteValueW failed: %w", err)
	}
	return nil
}

func nativeBroadcastEnvironmentChange() {
	environment, _ := windows.UTF16PtrFromString("Environment")
	var result uintptr

	_, _, _ = procSendMessageTimeoutW.Call(
		0xFFFF, // HWND_BROADCAST
		_WM_SETTINGCHANGE,
		0,
		uintptr(unsafe.Pointer(environment)),
		_SMTO_ABORTIFHUNG,
		5000,
		uintptr(unsafe.Pointer(&result)),
	)
}

const (
	scoopScopeUser   = "user"
	scoopScopeGlobal = "global"
)
