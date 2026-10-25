//go:build windows

package main

import (
	"os"
	"syscall"
	"unsafe"
)

const enableVirtualTerminalProcessing = 0x0004

// enableVirtualTerminal turns on ANSI escape sequence handling for the console,
// which is used to redraw the availability response in place.
func enableVirtualTerminal() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getConsoleMode := kernel32.NewProc("GetConsoleMode")
	setConsoleMode := kernel32.NewProc("SetConsoleMode")

	handle := os.Stdout.Fd()
	var mode uint32
	ok, _, _ := getConsoleMode.Call(handle, uintptr(unsafe.Pointer(&mode)))
	if ok == 0 {
		return
	}
	setConsoleMode.Call(handle, uintptr(mode|enableVirtualTerminalProcessing))
}
