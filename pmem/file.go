package pmem

import (
	"encoding/binary"
	"log"
	"os"
)

type FileRegion struct {
	name string
	fd   *os.File
	base int64
}

func (m *FileRegion) Read32(offset int64) uint32 {
	var b [4]byte
	n, err := m.fd.ReadAt(b[:], offset+m.base)
	if err != nil || n != 4 {
		log.Fatal(err)
	}
	return binary.LittleEndian.Uint32(b[:])
}

func (m *FileRegion) Read8(offset int64) uint8 {
	var b [1]byte
	n, err := m.fd.ReadAt(b[:], offset+m.base)
	if err != nil || n != 1 {
		log.Fatal(err)
	}
	return b[0]
}

func (m *FileRegion) Write32(offset int64, val uint32) {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], val)
	n, err := m.fd.WriteAt(b[:], offset+m.base)
	if err != nil || n != 4 {
		log.Fatal(err)
	}

}

func (m *FileRegion) Write8(offset int64, val uint8) {
	n, err := m.fd.WriteAt([]byte{val}, offset+m.base)
	if err != nil || n != 1 {
		log.Fatal(err)
	}
}

func (m *FileRegion) Name() string { return m.name }

func (m *FileRegion) Mem() []byte {
	log.Fatalf("file region %s cannot return mapping\n", m.name)
	return nil
}

func FileMap(name string, hwaddr int64, write bool, ioLen int64) Region {
	if ioLen == 0 {
		ioLen = 4096
	}
	ioLen += (-ioLen) & 4095 // round-up to number of pages
	var region FileRegion
	region.name = name
	region.base = hwaddr
	region.fd, _ = getFile(write)
	return &region
}
