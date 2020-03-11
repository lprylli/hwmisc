// +build !noasm

package pmem

import "unsafe"

//go:noinline
//go:nosplit
func Read32Go(ptr *uint32) uint32 {
	return *ptr
}

//go:noinline
//go:nosplit
func Read8Go(ptr *uint8) uint8 {
	return *ptr
}

//go:noinline
//go:nosplit
func Write32Go(ptr *uint32, val uint32) {
	*ptr = val
}

//go:noinline
//go:nosplit
func Write8Go(ptr *uint8, val uint8) {
	*ptr = val
}
func (m *MemRegion) Read32(off int64) uint32 {
	return Read32Go((*uint32)(unsafe.Pointer(&m.mem[off])))
}

func (m *MemRegion) Read8(off int64) uint8 {
	return Read8Go((*uint8)(unsafe.Pointer(&m.mem[off])))
}

func (m *MemRegion) Write32(off int64, val uint32) {
	*((*uint32)(unsafe.Pointer(&m.mem[off]))) = val
}

func (m *MemRegion) Write8(off int64, val uint8) {
	Write8Go(&m.mem[off], val)
}
