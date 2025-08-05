package ast

import (
	"encoding/binary"
	"fmt"
	"github.com/lprylli/hwmisc/pmem"
	"log"
	"time"
)

var SpiMode int

type spiChip struct {
	name      string
	Size      int64
	EraseSize int64
	op4b      bool
	mx3b4b    bool
}

type Fmc struct {
	mem                  *AstMem
	reg                  pmem.Region
	ce0CtlSlow           uint32
	ce0CtlFast           uint32
	id                   uint32
	Chip                 spiChip
	read, program, erase byte
}

const (
	FREAD      = 0x0b
	FREAD4B    = 0x0c
	PPROGRAM   = 0x02
	PPROGRAM4B = 0x12
	ERASE      = 0xd8
	ERASE4B    = 0xdc
	BRRD       = 0x16
	STATUS     = 0x05
	RDCR       = 0x15
	RFSR       = 0x70
)

func (f *Fmc) Sel(cmd byte) uint32 {
	if cmd == FREAD || cmd == FREAD4B {
		return f.ce0CtlFast
	} else {
		return f.ce0CtlSlow

	}
}

func (fmc *Fmc) spiXfer(out []byte, inSize int64) []byte {
	buf := make([]byte, inSize)
	fmc.reg.Write32(FMC_CE0CTL, fmc.Sel(out[0])|0x0003)
	firstBytes := len(out) & 3
	var s string
	if false {
		for x := 0; x < len(out) && x < 10; x++ {
			s += fmt.Sprintf("%02x ", out[x])
		}
		log.Printf("Spi: %s ...(out:%d in:%d)\n", s, len(out), inSize)
	}
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
	if SpiMode == 4 || cmd == FREAD4B || cmd == PPROGRAM4B || cmd == ERASE4B {
		out = make([]byte, 5+len(extra))
		binary.BigEndian.PutUint32(out[1:], uint32(off))
		out[0] = cmd
		copy(out[5:], extra)
	} else if SpiMode == 3 && (cmd == FREAD || cmd == PPROGRAM || cmd == ERASE) {
		if off&^0xffffff != 0 {
			log.Fatalf("spiXferAddr3B (cmd=%#x, offset=%#x)\n", cmd, off)
		}
		out = make([]byte, 4+len(extra))
		binary.BigEndian.PutUint32(out[0:], uint32(off))
		out[0] = cmd
		copy(out[4:], extra)
	} else {
		log.Fatalf("SpiXferAddr(cmd=%#x, offset=%#x): Unknown addr size (mode=%d)\n", cmd, off, SpiMode)
	}
	return fmc.spiXfer(out, inSize)
}

/*
check fast-read config before reenable
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
*/

// Reads SPI using ctl user-mode
func (fmc *Fmc) SpiRead(off, size int64) []byte {
	return fmc.spiXferAddr(fmc.read, uint32(off), []byte{0}, size)
}

func (fmc *Fmc) spiStatus() uint8 {
	buf := fmc.spiXfer([]byte{STATUS}, 4)
	return buf[0]
}

func (fmc *Fmc) spi4B() {
	switch {
	case fmc.Chip.mx3b4b:
		_ = fmc.spiXfer([]byte{0xB7}, 0)
	case fmc.id == 0x010220:
		_ = fmc.spiXfer([]byte{0x17, 0x80}, 0)
	default:
		log.Fatalf("don't know how to set spimode=4B on %#x\n", fmc.id)
	}
}
func (fmc *Fmc) spi3B() {
	switch {
	case fmc.Chip.mx3b4b:
		_ = fmc.spiXfer([]byte{0xE9}, 0)
	case fmc.id == 0x010220:
		_ = fmc.spiXfer([]byte{0x17, 0x00}, 0)
	default:
		log.Fatalf("don't know how to set spimode=3B on %#x\n", fmc.id)
	}
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
	_ = f.spiXferAddr(f.erase, uint32(off), nil, 0)
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
		f.spiXferAddr(f.program, uint32(off), buf[0:chunk], 0)
		status := f.spiWait()
		if status != 0 {
			log.Fatalf("While Page Programming 0x%x:  status = 0x%02x", off, status)
		}
		buf = buf[chunk:]
		off += chunk
	}
}

var spiDb = map[uint32]spiChip{
	0xc22019: {"mx25l25635f", 32 * 1024 * 1024, 64 * 1024, true, true},
	0xef4019: {"w25q256", 32 * 1024 * 1024, 64 * 1024, true, false},
	0x20ba20: {"n25q512a", 64 * 1024 * 1024, 64 * 1024, true, true},
	0xc2201a: {"mx66l51235l", 64 * 1024 * 1024, 64 * 1024, true, true},
	0x010220: {"s25fl512s", 64 * 1024 * 1024, 256 * 1024, true, false},
}

func (f *Fmc) is4B() bool {
	switch {
	case f.id == 0x20ba20:
		buf := f.spiXfer([]byte{RFSR}, 4)
		return buf[0]&0x01 != 0
	case f.id == 0x010220:
		buf := f.spiXfer([]byte{BRRD}, 4)
		return buf[0]&0x80 != 0
	case f.id == 0xef4019:
		buf := f.spiXfer([]byte{RDCR}, 4)
		return buf[0]&0x1 != 0
	case f.Chip.mx3b4b:
		buf := f.spiXfer([]byte{RDCR}, 4)
		return buf[0]&0x20 != 0
	default:
		log.Fatalf("is4B: Unknown chip id: %#x\n", f.id)
		return false
	}
}

func (a *AstHandle) FmcNew() *Fmc {
	_ = a.AstStop()
	fmc := &Fmc{
		reg:        Map("fmc", FMC_ADDR, true, 4096),
		mem:        Map("fmc-mem", FMC_MEM, true, 64*1024*1024).(*AstMem),
		ce0CtlSlow: 0x300,
		ce0CtlFast: 0x600,
	}
	fmc.id = fmc.spiId()
	if chip, ok := spiDb[fmc.id]; !ok {
		log.Fatalf("SpiId:0x%06x unknown!!\n", fmc.id)
	} else {
		fmc.Chip = chip
	}
	stat := fmc.spiStatus() &^ 0x40
	if stat&^2 != 0 {
		log.Fatalf("SpiStatus:0x%02x (spiid==0x%06x)\n", stat, fmc.id)
	}
	if stat&2 != 0 {
		fmc.writeDisable()
	}
	if SpiMode != 0 {
		switch SpiMode {
		case 4:
			fmc.spi4B()
			fmc.reg.Write32(0x04, 0x701)
		case 3:
			fmc.spi3B()
			fmc.reg.Write32(0x04, 0x700)
		default:
			log.Fatalf("Unknown spiMode:%d\n", SpiMode)
		}
	}
	astIs4B := (fmc.reg.Read32(0x04) & 1) != 0
	if astIs4B != fmc.is4B() {
		log.Fatalf("astIs4B(%v) != fmc.is4B(%v)\n", astIs4B, fmc.is4B())
	}
	if fmc.Chip.op4b {
		fmc.read, fmc.program, fmc.erase = FREAD4B, PPROGRAM4B, ERASE4B
	} else {
		fmc.read, fmc.program, fmc.erase = FREAD, PPROGRAM, ERASE
	}
	return fmc
}
