package msr

import (
	"encoding/binary"
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

const (
	CPUCTL_RDMSR = 0xc0106301
	CPUCTL_WRMSR = 0xc0106302
)

type MsrArg struct {
	msr  int64
	data uint64
}

func ioctlPtr64(fd uintptr, num uintptr, ptr *MsrArg) error {
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd, num, uintptr(unsafe.Pointer(ptr)))
	if err == 0 {
		return nil
	} else {
		return fmt.Errorf("ioctl(0x%x)-> errno=%d", num, err)
	}
}

func msrFile(cpu int) string {
	return fmt.Sprintf("/dev/cpuctl%d", cpu)
}

func readAt(f *os.File, b []byte, n int64) (int, error) {
	msr := MsrArg{msr: n}
	err := ioctlPtr64(f.Fd(), CPUCTL_RDMSR, &msr)
	binary.LittleEndian.PutUint64(b, msr.data)
	return 8, err
}

func writeAt(f *os.File, b []byte, n int64) (int, error) {
	msr := MsrArg{msr: n, data: binary.LittleEndian.Uint64(b)}
	return 8, ioctlPtr64(f.Fd(), CPUCTL_WRMSR, &msr)
}
