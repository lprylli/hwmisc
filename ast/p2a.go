package ast

import (
	"fmt"
	"log"
	"sync"

	"github.com/lprylli/hwmisc/pci"
	"github.com/lprylli/hwmisc/pmem"
)

type p2aHandle struct {
	bar1 pmem.Region
	name string
	m    sync.Mutex
	f004 uint32
}

var p2aGlob p2aHandle

type AstMem struct {
	p2a         *p2aHandle
	name        string
	start, size int64
}

var NoWrite bool
var Verbose bool
var Debug bool

func setIndex(p2a *p2aHandle, offset int64) {
	page := uint32(offset) & 0xffff0000
	if p2a.f004 != page {
		p2a.bar1.Write32(0xf004, page)
		p2a.bar1.Write32(0xf000, 1)
		p2a.f004 = page
	}
}

func (m *AstMem) Read32(offset int64) uint32 {
	if offset >= m.size {
		log.Printf("offset=%#x, %#v\n", offset, m)
		panic("out of bounds")
	}
	offset += m.start
	p2a := m.p2a
	p2a.m.Lock()
	defer p2a.m.Unlock()
	setIndex(p2a, offset)
	rc := p2a.bar1.Read32(0x10000 + offset&0xffff)
	if Debug {
		fmt.Printf("ast[0x%08x] -> 0x%08x\n", offset, rc)
	}
	return rc

}

func (m *AstMem) Read8(offset int64) uint8 {
	if offset >= m.size {
		log.Printf("offset=%#x, %#v\n", offset, m)
		panic("out of bounds")
	}
	offset += m.start
	p2a := m.p2a
	p2a.m.Lock()
	defer p2a.m.Unlock()
	setIndex(p2a, offset)
	rc := p2a.bar1.Read8(0x10000 + offset&0xffff)
	if Debug {
		fmt.Printf("ast[0x%08x] -> 0x%08x\n", offset, rc)
	}
	return rc

}

func (m *AstMem) Write32(offset int64, val uint32) {
	if offset >= m.size {
		panic("out of bounds")
	}
	offset += m.start
	if Debug {
		fmt.Printf("ast[0x%08x] := 0x%08x\n", offset, val)
	}
	p2a := m.p2a
	p2a.m.Lock()
	defer p2a.m.Unlock()
	setIndex(p2a, offset)
	if !NoWrite {
		p2a.bar1.Write32(0x10000+offset&0xffff, val)
	}
	_ = p2a.bar1.Read32(0x3cc)

}

func (m *AstMem) Write8(offset int64, val uint8) {
	if offset >= m.size {
		panic("out of bounds")
	}
	offset += m.start
	if Debug {
		fmt.Printf("ast[0x%08x].8 := 0x%08x\n", offset, val)
	}
	p2a := m.p2a
	p2a.m.Lock()
	defer p2a.m.Unlock()
	setIndex(p2a, offset)
	if !NoWrite {
		p2a.bar1.Write8(0x10000+offset&0xffff, val)
	}
	_ = p2a.bar1.Read32(0x3cc)

}

func (m *AstMem) Name() string { return m.name }

func (m *AstMem) Mem() []byte {
	log.Printf("Mem() function not implemented for p2a\n")
	panic("oops")
}

func Map(name string, hwaddr int64, write bool, ioLen int64) pmem.Region {
	if ioLen == 0 {
		ioLen = 4096
	}
	if arch() == "armv6l" {
		return pmem.Map(name, hwaddr, write, ioLen)
	}
	p2a := &p2aGlob
	p2a.m.Lock()
	defer p2a.m.Unlock()
	if p2a.bar1 == nil {
		w := pci.PciInit()
		l := w.FindById(0x1a03, 0x2000)
		if len(l) != 1 {
			log.Fatalf("Cannot find pci 1a03:2000 for acess to ast")
		}
		bar1Addr := l[0].Read32(0x14) &^ 0x1f
		p2a.bar1 = pmem.Map("ASTVGA-BAR1", int64(bar1Addr), true, 0x20000)
	}
	return &AstMem{p2a: p2a, start: hwaddr, size: ioLen}
}
