package pci

import (
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"syscall"
	"unsafe"
)

/*
 #include <sys/types.h>
 #include <sys/pciio.h>

*/
//import "C"

type PciDev struct {
	BasePciDev
	sel struct_pcisel
}

func Scan() *World {
	var w World
	var req struct_pci_conf_io
	var array [2048]struct_pci_conf

	req.match_buf_len = uint32(binary.Size(array))
	req.matches = &array[0]
	f, err := os.OpenFile("/dev/pci", os.O_RDWR, 0666)
	if err != nil {
		log.Fatalln(err)
	}
	w.fd = f

	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(f.Fd()),
		uintptr(PCIOCGETCONF),
		uintptr(unsafe.Pointer(&req)),
	)
	if errno != 0 {
		log.Fatalf("PCIOCGETCONF returns error %d\n", errno)
	}
	if req.status != PCI_GETCONF_LAST_DEVICE {
		log.Fatalf("PCIOCGETCONF.status=%d", req.status)
	}
	// log.Printf("matches:%d", req.num_matches)
	for i := 0; i < int(req.num_matches); i++ {
		p := &array[i]
		d := &PciDev{}
		d.domain = int(p.pc_sel.pc_domain)
		d.bus = int(p.pc_sel.pc_bus)
		d.devFn = int(p.pc_sel.pc_dev)*8 + int(p.pc_sel.pc_func)
		d.Name = fmt.Sprintf("%04x:%02x:%02x.%x", d.domain, d.bus, d.devFn/8, d.devFn%8)
		d.sel = p.pc_sel
		w.AddDev(d)
	}
	return &w
}

func (d *PciDev) Read(off int, l int) []byte {
	//log.Printf("%s:Read(%#02x,%d)", d.Name, off, l)
	if d.devType == -1 && off >= 0x100 {
		zero := [4]byte{0, 0, 0, 0}
		return zero[0:l]
	}
	var req struct_pci_io
	req.pi_sel = d.sel
	req.pi_width = int32(l)
	req.pi_reg = int32(off)
	buf := make([]byte, 4)
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(d.w.fd.Fd()),
		uintptr(PCIOCREAD),
		uintptr(unsafe.Pointer(&req)),
	)
	if errno != 0 {
		log.Fatalf("%s:PCIOCREAD(%d,%d):%d\n", d.Name, off, l, errno)
	}
	/*
		if err != nil || n != l {
			if !(n == 0 && off == 256 && err == io.EOF) {
				log.Fatalf("Read(%d, %d)->(%d, %s)\n", off, l, n, err)
			}
		}
	*/
	le.PutUint32(buf, uint32(req.pi_data))
	return buf
}

func (d *PciDev) Write(off int, data []byte) {

	var req struct_pci_io
	req.pi_sel = d.sel
	req.pi_width = int32(len(data))
	req.pi_reg = int32(off)
	var data4 [4]byte
	copy(data4[:], data)
	req.pi_data = uint32(le.Uint32(data4[:]))
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(d.w.fd.Fd()),
		uintptr(PCIOCWRITE),
		uintptr(unsafe.Pointer(&req)),
	)
	if errno != 0 {
		log.Fatalf("%s:PCIOCWRITE(%d,%d):%d\n", d.Name, off, len(data), errno)
	}
	/*
		if err != nil || n != len {
			if !(n == 0 && off == 256 && err == io.EOF) {
				log.Fatalf("Read(%d, %d)->(%d, %s)\n", off, len, n, err)
			}
		}
	*/
}

func (d *PciDev) Uncache() {

}
