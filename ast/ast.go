package ast

import (
	"fmt"
	"log"
	"runtime"
	"time"

	"github.com/lprylli/hwmisc/pmem"
)

var SocReset, LpcReset bool

type AstHandle struct {
	scu      pmem.Region
	I2c      pmem.Region
	wdt      pmem.Region
	lpc      pmem.Region
	dram     pmem.Region
	mac      [2]pmem.Region
	Ast2500  bool
	ThisIsMe bool // true if program is running on AST itself
}

func New() *AstHandle {
	var a AstHandle
	a.scu = Map("scu", SCU_ADDR, true, 4096)
	a.Ast2500 = isAst2500(a.scu)
	a.wdt = Map("wdt", WDT_ADDR, true, 4096)
	a.I2c = Map("i2c", I2C_ADDR, true, 4096)
	a.lpc = Map("lpc", LPC_ADDR, true, 4096)
	a.mac[0] = Map("mac0", MAC0_ADDR, true, 4096)
	a.mac[1] = Map("mac1", MAC1_ADDR, true, 4096)

	a.ThisIsMe = (arch() == "armv6l")
	return &a
}

func (a *AstHandle) Dram() pmem.Region {
	if a.dram == nil {
		a.dram = Map("dram", DRAM_ADDR, false, 512*1024*1024)
	}
	return a.dram
}

func isAst2500(scu pmem.Region) (ast2500 bool) {

	astId := scu.Read32(SCU_REVID)
	switch astId &^ 0x0f0000 {
	case 0x02000303:
		ast2500 = false
	case 0x04000303:
		ast2500 = true
	default:
		log.Fatalf("Don't know how to reset chip with id: 0x%08x\n", astId)
	}
	return
}

type AstPrevState struct {
	rst70State uint32
	wdt2Prev   uint32
}

// Return a bitmask of which i2c busses have their pin enabled.
func (a *AstHandle) I2cEnabledSet() uint32 {
	i2cEnabled := (a.scu.Read32(0x90) & 0xfff0000) >> 14
	if a.Ast2500 {
		scuA4 := a.scu.Read32(0xA4)
		i2cEnabled |= (scuA4&0x8000)>>14 + (scuA4&0x2000)>>13
	} else {
		i2cEnabled |= 3
	}
	return i2cEnabled
}

func I2cBase(i int) (base int64) {
	base = int64(i*0x40 + 0x40)
	if i >= 7 {
		base = int64((i-7)*0x40 + 0x300)
	}
	return
}

// Only supports AST2400 and AST2500
func (a *AstHandle) AstStop() (prev AstPrevState) {
	if a.ThisIsMe {
		log.Fatal("Cannot execute op requiring ast soc stop on myself\n")
	}
	log.Printf("Stopping AST\n")
	scu := a.scu
	wdt := a.wdt

	scu.Write32(0, 0x1688a8a8)
	_ = scu.Read32(0x70)
	prev.wdt2Prev = wdt.Read32(0x2c)
	wdt.Write32(0xc, 0)  //dis WDT1
	wdt.Write32(0x2c, 0) //dis WDT2
	if a.Ast2500 {
		wdt.Write32(0x4c, 0) //dis WDT3
	}
	prev.rst70State = scu.Read32(0x70)
	scu.Write32(0x70, prev.rst70State|3) // dis cpu
	wdt.Write32(0xc, 0)                  //dis WDT1
	wdt.Write32(0x2c, 0)                 //dis WDT2
	wdt.Write32(0x34, 1)                 // clr WDT2 timeout/2nd boot status

	_ = scu.Read32(SCU_REVID)

	if a.Ast2500 {
		wdt.Write32(0x4c, 0) //dis WDT3
		wdt.Write32(0x54, 1) // clr WDT3 timeout/2nd boot status
	}
	i2cEnabled := a.I2cEnabledSet()
	i2c := a.I2c
	for i := 0; i < 14; i++ {
		base := I2cBase(i)
		if i2cEnabled&(1<<uint(i)) != 0 && i2c.Read32(base+0) != 0 {
			i2c.Write32(base+0, 0)
			i2c.Write32(base+0x14, 0)
			_ = i2c.Read32(base + 0x14)
		}
	}
	lpc := a.lpc
	// disable lpc channels
	lpc.Write32(0, lpc.Read32(0)&^0xec)
	// disable kcs/bt
	lpc.Write32(0x10, lpc.Read32(0x10)&^0x5)
	return prev
}

func (a *AstHandle) AstRestart(prev AstPrevState) {
	log.Printf("Restarting AST\n")
	scu := a.scu
	wdt := a.wdt

	scu.Write32(0, 0x1688a8a8)
	if a.Ast2500 {
		scu.Write32(0x7c, 0x3001) // disable-spi, enable-cpu-boot
		scu.Write32(0x70, 0x1000) // enable spi-master
	} else {
		scu.Write32(0x70, prev.rst70State)
	}

	if a.Ast2500 {
		_ = wdt.Read32(0x1c)
		resetMask := uint32(0x033fdff3)
		if LpcReset {
			resetMask |= 0x00002000
		}
		log.Printf("ResetMask=0x%08x\n", resetMask)
		wdt.Write32(0x1c, resetMask) // reset-mask
	}
	wdt.Write32(0x2c, prev.wdt2Prev)
	wdt.Write32(0x4, 0x10)   // WDT1 reload-value = 0x10
	wdt.Write32(0x8, 0x4755) // restart WDT1
	if SocReset {
		wdt.Write32(0xc, 0x33) // soc-reset + enable-signal + reset-system at timeout

	} else {
		wdt.Write32(0xc, 0x13) // enable-signal + reset-system at timeout
	}
}

func (a *AstHandle) AstReset() {
	prev := a.AstStop()
	if prev.rst70State&3 != 2 {
		prev.rst70State = (prev.rst70State &^ 3) | 2
	}
	a.AstRestart(prev)
}

func (a *AstHandle) AstResume() {
	a.AstRestart(AstPrevState{rst70State: (a.scu.Read32(0x70) &^ 3) | 2, wdt2Prev: 0x10})
}

func (a *AstHandle) AstI2c3Speed() {
	i2c := a.I2c
	for {
		ctl104 := i2c.Read32(0x104)
		if (ctl104 & 0xf) == 5 {
			new := (ctl104 &^ 0xf) | 0xb
			i2c.Write32(0x104, new)
			log.Printf("switch i2c[0x104] from 0x%#08x to 0x%#08x\n", ctl104, new)
		}
		runtime.Gosched()
	}
}

func (a *AstHandle) AstI2c3Disable() {
	const i2c3Enable = 0x20000
	scu := a.scu
	expire := time.Now().Add(120 * time.Second)
	var scu90 uint32
	for !time.Now().After(expire) {
		scu90 = scu.Read32(0x90)
		if (scu90 & i2c3Enable) == i2c3Enable {
			new := (scu90 &^ i2c3Enable)
			scu.Write32(0x90, new)
			log.Printf("switch scu[0x90] from 0x%#08x to 0x%#08x\n", scu90, new)
		}
		//runtime.Gosched()
		time.Sleep(100 * time.Microsecond)
	}
	if scu90&i2c3Enable != 0 {
		fmt.Printf("PMbus not disabled\n")
	} else {
		fmt.Printf("PMbus is now disabled\n")
	}
}

func AstInfo() (string, int) {
	scu := Map("scu", SCU_ADDR, false, 4096)
	id := scu.Read32(SCU_REVID)
	fmt.Printf("ast silicon rev id:0x%08x\n", id)
	step := (id >> 16) & 0xff
	if step == 3 {
		step = 2
	} else if step == 2 {
		step = 99
	}
	id &^= 0xff0000
	var s string
	switch id {
	case 0x04000303:
		s = "2500"
	case 0x02000303:
		s = "2400"
	case 0x04000103:
		s = "2510"
	case 0x04000203:
		s = "2520"
	case 0x04000403:
		s = "2530"
	default:
		s = "-unknown-"
	}
	return s, int(step)
}

func bit(val uint32, pos uint8) uint32 {
	return (val >> pos) & 1

}

func bits(val uint32, pos uint8, num uint8) uint32 {
	return (val >> pos) & ((1 << num) - 1)

}

var spiReg = `
0 CEType
  16 CE write type
  1:0 CE Flash type (2 == spiflash)
4 CeCtl
  8 Ce0Div2
  0 Ce0Addr4B
0x10 Ce0Ctl
  30:28 IO mode
  23:16 Cmd
  15 dummy cycles bit2
  12 CmdMerge
  11:8 ClkSpeed (0 == HCLK/16, 15 = HCLK)
  7:6 dummy cycles bit1:0
  5 Msbit-Lsbit order
  3 DualInput
  2 CeStopActive
  1:0 CmdMode
0x94 DataInputDelay
`

var spiRegs = ParseRegs(spiReg)

func SpiInfo() {
	spi := Map("spi", FMC_ADDR, true, 4096)
	chip, _ := AstInfo()
	if chip != "2500" {
		log.Printf("Only accept AST2500, chip=%s\n", chip)
		return
	}
	fmc0 := spi.Read32(0)
	fmcA0 := spi.Read32(0xa0)
	fmt.Printf("spi0=0x%x, writable=%d filter=%d\n", fmc0, bit(fmc0, 16), bit(fmcA0, 0))
	fmt.Printf("\tEnables=0x%x\n", spi.Read32(0xA0))
	if fmc0&3 != 2 {
		log.Fatal("FMC/CE0 !")
	}
	fmc4 := spi.Read32(4)
	fmt.Printf("div2timings=%d addr_bytes=%d\n", bit(fmc4, 8), 3+bit(fmc4, 0))

	fmc10 := spi.Read32(0x10)
	fmt.Printf("iomode=0x%x cmd=0x%x\n", bits(fmc10, 28, 3), bits(fmc10, 16, 8))

	for _, r := range spiRegs {
		val := spi.Read32(r.Off)
		for _, f := range r.Fields {
			fmt.Printf("%s.%s: 0x%x\n", r.Name, f.Name, bits(val, f.FirstBit, f.NumBits))
		}
		if len(r.Fields) == 0 {
			fmt.Printf("%s: 0x%x\n", r.Name, val)

		}
	}

}
