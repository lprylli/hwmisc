package pmem

import (
	"log"
	"os"
	"syscall"
)

type Region interface {
	Name() string
	Read32(offset int64) uint32
	Read8(offset int64) uint8
	Write32(offset int64, val uint32)
	Write8(offset int64, val uint8)
	Mem() []byte
}

type MemRegion struct {
	name string
	mem  []byte
}

func (m *MemRegion) Name() string { return m.name }

func (m *MemRegion) Mem() []byte { return m.mem }

type iomapping struct {
	hwAddr int64
	len    int64
	write  bool
}

var hwMaps = make(map[iomapping]Region)
var memRW, memRO *os.File

var DevName = "/dev/mem"

func getFile(write bool) (f *os.File, prot int) {
	var memPtr **os.File
	var openFlag int

	if write {
		memPtr = &memRW
		openFlag = os.O_RDWR
		prot = syscall.PROT_WRITE | syscall.PROT_READ
	} else {
		memPtr = &memRO
		openFlag = os.O_RDONLY
		prot = syscall.PROT_READ
	}
	if *memPtr == nil {
		var err error
		*memPtr, err = os.OpenFile(DevName, openFlag, 0666)
		if err != nil {
			log.Fatalf("%s:%s\n", DevName, err)
		}
	}
	f = *memPtr
	return
}

func memHandle(m iomapping) []byte {

	f, prot := getFile(m.write)
	data, err := syscall.Mmap(int(f.Fd()), m.hwAddr, int(m.len), prot, syscall.MAP_SHARED)
	//log.Printf("mmap=%p", data)
	if err != nil {
		log.Fatalln(err)
	}
	return data
}

func Map(name string, hwaddr int64, write bool, ioLen int64) Region {
	if ioLen == 0 {
		ioLen = 4096
	}
	ioLen += (-ioLen) & 4095 // round-up to number of pages
	m := iomapping{hwaddr, ioLen, write}
	data := hwMaps[m]
	if data != nil {
		return data
	}
	data = &MemRegion{name: name, mem: memHandle(m)}
	hwMaps[m] = data
	return data
}
