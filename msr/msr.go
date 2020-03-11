package msr

import (
	"encoding/binary"
	"fmt"
	"log"
	"os"
)

var msrFdMap map[int]*os.File

func getFd(cpu int) *os.File {
	var err error
	if cpu == -1 {
		cpu = 0
	}
	fd, ok := msrFdMap[cpu]
	if !ok {
		fd, err = os.OpenFile(msrFile(cpu), os.O_RDWR, 0666)
		if err != nil {
			log.Fatal(err)
		}
	}
	return fd
}

func Read(cpu int, addr uint32) (uint64, error) {
	fd := getFd(cpu)
	var b [8]byte
	n, err := readAt(fd, b[:], int64(addr))
	if err != nil {
		return 0, err
	}
	if n != 8 {
		return 0, fmt.Errorf("msr read 0x%x got %d bytes", addr, n)
	}
	return binary.LittleEndian.Uint64(b[:]), nil
}

func Write(cpu int, addr uint32, val uint64) error {
	fd := getFd(cpu)
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], val)
	n, err := writeAt(fd, b[:], int64(addr))
	if err != nil {
		return err
	}
	if n != 8 {
		return fmt.Errorf("msr write 0x%x got %d bytes", addr, n)
	}
	return nil
}

func MustRead(cpu int, addr uint32) uint64 {
	val, err := Read(cpu, addr)
	if err != nil {
		log.Fatal(err)
	}
	return val
}
