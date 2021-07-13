package ast

import (
	"encoding/binary"
	"log"
	"time"

	"github.com/lprylli/hwmisc/pmem"
)

var Mode4B bool = true

type Fmc struct {
	mem        *AstMem
	reg        pmem.Region
	ce0CtlSlow uint32
	ce0CtlFast uint32
	Size       int64
}

func (f *Fmc) Sel(cmd byte) uint32 {
	if cmd == 0x0b {
		return f.ce0CtlFast
	} else {
		return f.ce0CtlSlow

	}
}

func (fmc *Fmc) spiXfer(out []byte, inSize int64) []byte {
	buf := make([]byte, inSize)
	fmc.reg.Write32(FMC_CE0CTL, fmc.Sel(out[0])|0x0003)
	firstBytes := len(out) & 3
	for x := 0; x < firstBytes; x++ {
		fmc.mem.Write8(0, out[x])
	}
	for x := firstBytes; x < len(out); x += 4 {
		fmc.mem.Write32(0, binary.LittleEndian.Uint32(out[x:]))
	}
	for x := int64(0); x < inSize; x += 4 {
		val := fmc.mem.Read32(x)
		binary.LittleEndian.PutUint32(buf[x:], val)
	}
	fmc.reg.Write32(FMC_CE0CTL, fmc.Sel(out[0])|0x0007)
	return buf
}

func (fmc *Fmc) spiXferAddr(cmd byte, off uint32, extra []byte, inSize int64) []byte {
	var out []byte
	if Mode4B {
		out = make([]byte, 5+len(extra))
		binary.BigEndian.PutUint32(out[1:], uint32(off))
		out[0] = cmd
		copy(out[5:], extra)
	} else {
		out = make([]byte, 4+len(extra))
		binary.BigEndian.PutUint32(out[0:], uint32(off))
		out[0] = cmd
		copy(out[4:], extra)
	}
	return fmc.spiXfer(out, inSize)
}

// Reads SPI using fast-read command.
func (fmc *Fmc) SpiReadMapped(off, size int64) []byte {
	var b = make([]byte, size)
	fmc.reg.Write32(FMC_CE0CTL, fmc.ce0CtlFast|0x0b0041)
	for x := int64(0); x < size; x += 4 {
		val := fmc.mem.Read32(off + x)
		binary.LittleEndian.PutUint32(b[off+x:], val)
	}
	return b
}

// Reads SPI using ctl user-mode
func (fmc *Fmc) SpiRead(off, size int64) []byte {
	return fmc.spiXferAddr(0x0b, uint32(off), []byte{0}, size)
}

func (fmc *Fmc) spiStatus() uint8 {
	buf := fmc.spiXfer([]byte{5}, 4)
	return buf[0]
}

func (fmc *Fmc) spi4B() {
	_ = fmc.spiXfer([]byte{0xB7}, 0)
}
func (fmc *Fmc) spi3B() {
	_ = fmc.spiXfer([]byte{0xE9}, 0)
}

// Reads SPI-ID
func (fmc *Fmc) spiId() uint32 {
	buf := fmc.spiXfer([]byte{0x9f}, 4)
	return uint32(buf[0])<<16 + uint32(buf[1])<<8 + uint32(buf[2])
}

func (f *Fmc) spiWait() uint8 {
	var s uint8
	expire := time.Now().Add(10 * time.Second)
	for s := f.spiStatus(); s&1 != 0; s = f.spiStatus() {
		time.Sleep(10 * time.Microsecond)
		if time.Now().After(expire) {
			log.Fatalf("timeout waiting for operation to finish, stat = 0x%02x", s)
		}
	}
	return s
}

func (f *Fmc) writeEnable() {
	_ = f.spiXfer([]byte{0x6}, 0)
}

func (f *Fmc) writeDisable() {
	_ = f.spiXfer([]byte{0x4}, 0)
}

func (f *Fmc) EraseBlock(off int64) {
	f.writeEnable()
	_ = f.spiXferAddr(0xd8, uint32(off), nil, 0)
	status := f.spiWait()
	if status != 0 {
		log.Fatalf("While erasing sector 0x%x:  status = 0x%02x", off, status)
	}
}

func (f *Fmc) Write(off64 int64, buf []byte) {
	off := int(off64)
	for len(buf) > 0 {
		chunk := 256 - (off & 0xff)
		if chunk > len(buf) {
			chunk = len(buf)
		}
		f.writeEnable()
		f.spiXferAddr(0x2, uint32(off), buf[0:chunk], 0)
		status := f.spiWait()
		if status != 0 {
			log.Fatalf("While Page Programming 0x%x:  status = 0x%02x", off, status)
		}
		buf = buf[chunk:]
		off += chunk
	}
}

func (a *AstHandle) FmcNew() *Fmc {
	_ = a.AstStop()
	fmc := &Fmc{
		reg:        Map("fmc", FMC_ADDR, true, 4096),
		mem:        Map("fmc-mem", FMC_MEM, true, 64*1024*1024).(*AstMem),
		ce0CtlSlow: 0x300,
		ce0CtlFast: 0x600,
		Size:       32 * 1024 * 1024,
	}
	id := fmc.spiId()
	switch id {
	case 0xc22019:
	case 0x20ba20:
		/* nothing */
	case 0xc2201a:
		fmc.Size = 64 * 1024 * 1024
	default:
		log.Fatalf("SpiId:0x%06x unknown!!\n", id)

	}
	if Mode4B {
		fmc.reg.Write32(0x04, 0x701)
	} else {
		fmc.reg.Write32(0x04, 0x700)
	}
	stat := fmc.spiStatus()
	if stat&^2 != 0 {
		log.Fatalf("SpiStatus:0x%02x\n", stat)
	}
	if stat&2 != 0 {
		fmc.writeDisable()
	}
	if Mode4B {
		fmc.spi4B()
	} else {
		fmc.spi3B()
	}
	return fmc
}
