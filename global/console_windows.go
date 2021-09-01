//go:build windows
// +build windows

package global

import (
	"golang.org/x/sys/windows"
	"os"
	"syscall"
	"unsafe"
)

// DisableConsoleQuickEditMode Turn off Windows Console's QuickEdit Mode
func DisableConsoleQuickEditMode() {
	stdin := windows.Handle(os.Stdin.Fd())

	var originalMode uint32
	windows.GetConsoleMode(stdin, &originalMode)

	newMode := originalMode &^ windows.ENABLE_QUICK_EDIT_MODE
	windows.SetConsoleMode(stdin, newMode)
}

// SetConsoleTitle Set windows console title
func SetConsoleTitle(title string) (int, error) {
	handle, err := syscall.LoadLibrary("Kernel32.dll")
	if err != nil {
		return 0, err
	}
	defer syscall.FreeLibrary(handle)
	proc, err := syscall.GetProcAddress(handle, "SetConsoleTitleW")
	if err != nil {
		return 0, err
	}
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	r, _, err := syscall.Syscall(proc, 1, uintptr(unsafe.Pointer(titlePtr)), 0, 0)
	return int(r), err
}
