package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"hash/crc32"
	"io/ioutil"
	"log"
	"math"
	"strings"

	"github.com/lprylli/hwmisc/ast"
	"github.com/lprylli/hwmisc/pmem"
)

func len64(b []byte) int64 { return int64(len(b)) }

func spiMakeEnv(b []byte, env string) {
	var sector []byte
	if len(b) == 0x2000000 {
		sector = b[0x1fc0000:0x1fd0000]
	} else if len(b) == 0x10000 {
		sector = b
	} else {
		log.Fatalf("Where to put env in image len 0x%x?!!\n", len(b))
	}
	l := strings.Split(env, "\n")
	for _, c := range sector {
		if c != 0xff {
			log.Fatalf("Env sector not originally cleared\n")
		}
	}
	sector[4] = 1
	pos := 5
	for _, v := range l {
		for _, c := range []byte(v) {
			sector[pos] = c
			pos++
		}
		sector[pos] = 0
		pos++
	}
	sector[pos] = 0
	pos++
	crc := crc32.ChecksumIEEE(sector[5:])
	binary.LittleEndian.PutUint32(sector[0:], crc)
}

func doSpiWrite(spiWrite string, spiOff int64, spiLen int64, spiEnv string) {
	buf, err := ioutil.ReadFile(spiWrite)
	if err != nil {
		log.Fatal(err)
	}
	if spiEnv != "" {
		spiMakeEnv(buf, spiEnv)
	}
	if spiLen > 0 && spiLen > len64(buf) {
		log.Fatalf("%s too small for -spilen 0x%x\n", spiWrite, spiLen)
	} else if spiLen <= 0 {
		spiLen = len64(buf)
	}
	if spiOff&0xffff != 0 {
		log.Fatalf("Cannot write non-block aligned data to flash")
	}
	a := ast.New()
	fmc := a.FmcNew()
	for x := int64(0); x < spiLen; x += 0x10000 {
		sec := x + spiOff
		chunk := int64(0x10000)
		if chunk > spiLen-x {
			chunk = spiLen - x
		}
		oldData := fmc.SpiRead(x+spiOff, chunk)
		if bytes.Compare(oldData, buf[x:x+chunk]) == 0 {
			fmt.Printf("\rSkipping sector 0x%x: no change", sec)
			continue
		}
		fmt.Printf("\rErasing sector 0x%07x   (%2d%%)          ", sec, x*100/spiLen)
		fmc.EraseBlock(sec)
		fmt.Printf("\rWriting sector 0x%07x   (%2d%%)          ", sec, (x*100+50)/spiLen)

		fmc.Write(sec, buf[x:x+chunk])
	}
	fmt.Printf("\nVerifying...\n")
	reread := fmc.SpiRead(spiOff, spiLen)
	if bytes.Compare(reread, buf[0:spiLen]) != 0 {
		log.Fatalf("Reread comparison failed!\n")
	}
}

const (
	I2C_IDLE = 0
	MACTIVE  = 8
	MTXD     = 12
	MRXACK   = 13
	MRX      = 14
	MTXACK   = 15
)

type i2cCtxt struct {
	poolBase int64
	a        *ast.AstHandle
	dmaAddr  uint32
	dmaLen   uint32
	rx       bool
	cmd      uint32
	rbytes   uint32
	bus      int
	dram     pmem.Region
}

var stateStrs = []string{"idle", "swait", "2", "recover", "srx", "stxack", "stxd", "srxack", "mactive", "mstart", "mstartr", "mstop", "mtxd", "mrxack", "mrx", "mtxack"}

func i2cNotSupported(state, cmd uint32) {
	log.Printf("X \n i2c monitoring/decoder: not yet supported i2c controller sequence:state=%s,cmd=%#x", stateStrs[state], cmd)

}

func i2cSmart(c []pmem.GpioChange) {
	for i, x := range c {
		ctxt := x.Mon.Aux.(*i2cCtxt)
		state := (x.New >> 19) & 0xf
		cmd := x.New & 0x3ff
		switch state {
		case MTXD:
			if cmd != 2 && cmd != 0x102 && cmd != 0x122 {
				i2cNotSupported(state, cmd)
				pmem.GpioRes(c[i : i+1])
			} else {
				fmt.Printf("T%d %02x ", ctxt.bus, x.Aux[0]&0xff)
			}
		case MTXACK:
			if cmd != 0x98 && cmd != 0x238 && cmd != 0x20 {
				i2cNotSupported(state, cmd)
				pmem.GpioRes(c[i : i+1])
			} else {
				var b [4]uint8
				binary.LittleEndian.PutUint32(b[:], x.Aux[1])
				rxptr := x.Aux[0] >> 24
				if (rxptr & 0x1f) == 0 {
					log.Fatalf("MTXACK state: bufpool ptrs = %#08x", x.Aux[0])
				}
				//fmt.Printf("R %02x (%#08x %#08x %#08x %#08x)", b[(rxptr-1)&3], x.New, x.Aux[0], x.Aux[1], x.Aux[2])
				fmt.Printf("R%d %02x ", ctxt.bus, b[(rxptr-1)&3])
			}
		case I2C_IDLE:
			fmt.Printf("\n")
		}
	}
}

func i2cExtra(c *pmem.GpioChange) {
	ctxt := c.Mon.Aux.(*i2cCtxt)
	a := ctxt.a
	state := (c.New >> 19) & 0xf
	cmd := c.New & 0x3ff
	base := c.Addr &^ 0x3f
	switch state {
	case MRX:
		if !ctxt.rx {
			ctxt.dmaAddr = a.I2c.Read32(base + 0x24)
			ctxt.dmaLen = a.I2c.Read32(base + 0x28)
			ctxt.cmd = cmd
			ctxt.rbytes = 0
			ctxt.rx = true
		}
	case MTXD:
		c.Aux[0] = a.I2c.Read32(base + 0x20)
	case MTXACK:
		if ctxt.cmd == 0x98 {
			c.Aux[0] = a.I2c.Read32(base + 0x1c)
			rxptr := c.Aux[0] >> 24
			poolOff := ctxt.poolBase + int64((rxptr-1)&^3)
			c.Aux[1] = a.I2c.Read32(poolOff)
			c.Aux[2] = uint32(poolOff)
		} else if ctxt.cmd == 0x238 || ctxt.cmd == 0x20 {
			if !ctxt.rx || ctxt.rbytes >= ctxt.dmaLen {
				log.Fatalf("MTXACK, ctxt: %#v\n", ctxt)
			}
			c.Aux[1] = ctxt.dram.Read32(int64((ctxt.dmaAddr + ctxt.rbytes) & ^uint32(3)))
			c.Aux[0] = (ctxt.rbytes + 1) << 24
			c.Aux[2] = math.MaxUint32
			ctxt.rbytes += 1
		} else {
			ctxt.rx = false
			i2cNotSupported(state, cmd)

		}
	default:
		ctxt.rx = false

	}
}

func i2cDec(c []pmem.GpioChange) {
	for i, x := range c {
		state := (x.New >> 19) & 0xf
		if x.Addr&0x3f == 0x14 {
			fmt.Printf("t=%d SCL=%d,SDA=%d,cmd=%#x state=%s\n", x.Delay, (x.New>>18)&1, (x.New>>17)&1, x.New&0x3ff, stateStrs[state])
		} else {
			pmem.GpioRes(c[i : i+1])
		}
	}
}

var i2cMonRaw bool

func doI2cMon(bus int) {
	a := ast.New()
	i2c := a.I2c
	dram := a.Dram()
	var mons []*pmem.Monitor
	i2cEnabled := a.I2cEnabledSet()
	for i := uint32(0); i < 14; i++ {
		var ctxt i2cCtxt
		ctxt.a = a
		ctxt.dram = dram
		ctxt.bus = int(i)
		base := int64(i*0x40 + 0x40)
		if i >= 7 {
			base = int64((i-7)*0x40 + 0x300)
		}
		if (bus == -1 || uint32(bus) == i) && i2cEnabled&(1<<i) != 0 && i2c.Read32(base+0) != 0 {
			m := pmem.Monitor{M: i2c, Off: base}
			if a.Ast2500 {
				ctxt.poolBase = int64(0x200 + 16*i)
			} else {
				ctxt.poolBase = int64(0x800 + ((i2c.Read32(base)>>20)&7)*0x100)
			}
			if !i2cMonRaw {
				m.Mask = ^uint32(0x00780000)
				m.Off += 0x14
				m.Aux = &ctxt
				mons = append(mons, &m)
			} else {
				for _, off := range []int64{0x14, 0x18, 0x1c, 0x20, 0x24, 0x28} {
					m2 := m
					m2.Off += off
					mons = append(mons, &m2)

				}
			}

		}
	}
	if !i2cMonRaw {
		pmem.MonLoopFun(mons, i2cSmart, i2cExtra)
	} else {
		pmem.MonLoopFun(mons, i2cDec, nil)

	}
}

func main() {
	var astReset bool
	var i2c3Speed, i2c3Disable bool
	var spiRead, spiWrite, spiEnv string
	var spiOff, spiLen int64
	var i2cMon int
	var mii bool
	flag.BoolVar(&astReset, "reset", false, "Reset AST chip")
	flag.BoolVar(&ast.NoWrite, "noop", false, "Fake AST writes")
	flag.BoolVar(&ast.Verbose, "verbose", false, "Output each individual ast writes")
	flag.BoolVar(&ast.SocReset, "soc", false, "with -reset, reset full SOC")
	flag.BoolVar(&ast.LpcReset, "lpc", false, "with -reset, also reset LPC")
	flag.BoolVar(&i2c3Speed, "i2c3", false, "continuously monitor/adjust i2c speed for bus-3")
	flag.BoolVar(&i2c3Disable, "i2c3dis", false, "continuously monitor/disable i2c bus-3")
	flag.StringVar(&spiRead, "spiread", "", "file where to store flash contents")
	flag.Int64Var(&spiOff, "spioff", 0, "flash offset to read or write")
	flag.Int64Var(&spiLen, "spilen", -1, "size to read write")
	flag.StringVar(&spiWrite, "spiwrite", "", "file to write to flash")
	flag.StringVar(&spiEnv, "spienv", "", "env vars to write to flash (comma separated)")
	flag.IntVar(&i2cMon, "i2cmon", -2, "i2c bus to monitor (-1 == all)")
	flag.BoolVar(&i2cMonRaw, "i2cmonraw", false, "raw gpio mon for i2c")
	flag.BoolVar(&mii, "mii", false, "info about mii")

	flag.Parse()
	chip, step := ast.AstInfo()
	fmt.Printf("AST%s-A%d\n", chip, step)

	var a *ast.AstHandle
	if chip == "2400" || chip == "2500" {
		a = ast.New()
	}

	if mii {
		a.MiiInfo()
	}
	if i2c3Speed {
		a.AstI2c3Speed()
	}
	if i2c3Disable {
		a.AstI2c3Disable()
	}
	if astReset {
		if a.ThisIsMe {
			a.AstResume()
		} else {
			a.AstReset()
		}
	}
	if i2cMon != -2 {
		doI2cMon(i2cMon)
	}
	if spiRead != "" {
		fmc := a.FmcNew()
		if spiLen <= 0 {
			spiLen = fmc.Size
		}
		buf := fmc.SpiRead(spiOff, spiLen)
		err := ioutil.WriteFile(spiRead, buf, 0666)
		if err != nil {
			log.Fatal(err)
		}
	}
	if spiWrite != "" {
		doSpiWrite(spiWrite, spiOff, spiLen, spiEnv)
	}
}
