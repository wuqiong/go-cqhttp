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

func AdjustConsoleWindowSize(width int32, height int32) {
	getConsoleWindow := syscall.NewLazyDLL("kernel32.dll").NewProc("GetConsoleWindow")
	if getConsoleWindow.Find() != nil {
		return
	}
	getWindowRect := syscall.NewLazyDLL("User32.dll").NewProc("GetWindowRect")
	if getWindowRect.Find() != nil {
		return
	}
	moveWindow := syscall.NewLazyDLL("User32.dll").NewProc("MoveWindow")
	if moveWindow.Find() != nil {
		return
	}
	type RECT struct {
		left   int32
		top    int32
		right  int32
		bottom int32
	}
	consoleWindowHandle, _, _ := getConsoleWindow.Call()
	if consoleWindowHandle == 0 {
		return
	}
	var consoleRect RECT
	getWindowRect.Call(consoleWindowHandle, uintptr(unsafe.Pointer(&consoleRect)))
	var x = consoleRect.left
	var y = consoleRect.top
	var repaint int32 = 1
	moveWindow.Call(consoleWindowHandle, uintptr(x), uintptr(y),
		uintptr(width), uintptr(height),
		uintptr(repaint))
	return
}
