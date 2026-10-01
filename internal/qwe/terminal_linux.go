package qwe

import (
	"syscall"
	"unsafe"
)

func isTerminal(fd uintptr) bool {
	var term syscall.Termios
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, syscall.TCGETS, uintptr(unsafe.Pointer(&term)))
	return errno == 0
}
