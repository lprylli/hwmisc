package main

import (
	"bufio"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"unsafe"

	"github.com/lprylli/hwmisc/ast"
	"github.com/lprylli/hwmisc/pmem"
)

const (
	PgSize = 4096
	PgMask = ^(PgSize - 1)
)

func ReadFull(f io.Reader, b []byte) {
	o := 0
	n := 0
	var e error
	for o < len(b) {
		n, e = f.Read(b[o:])
		if e != nil || n <= 0 {
			break
		}
		o += n
	}
	if o != len(b) || e != nil {
		log.Fatalf("ReadFull(%d):%d,%s,prev=%d", len(b), n, e, o)
	}
}

var mapFn func(name string, hwaddr int64, write bool, ioLen int64) pmem.Region

func astGpioMon() {
	scuMap := mapFn("scu", ast.SCU_ADDR, false, 0)
	gpioMap := mapFn("gpio", ast.GPIO_ADDR, false, 0)

	var mons []*pmem.Monitor
	addMonitor := func(m pmem.Region, r int64, mask uint32) {
		mons = append(mons, &pmem.Monitor{M: m, Off: r, Mask: mask})
	}

	regs := []int64{0x0, 0x04, 0x20, 0x24, 0x70, 0x74, 0x78, 0x7c, 0x80, 0x84, 0x88, 0x8c}
	masks := make([]uint32, len(regs))
	hash := make(map[int64]int)
	for i, r := range regs {
		hash[r] = i
	}
	if false {
		masks[hash[0x0]] = 0xc0000
		masks[hash[0x70]] = 0x1bb00 | 0x40900 | 0xc1800 | 0x81100 | 0x301800
		masks[hash[0x78]] = 0x3c3f00
		masks[hash[0x80]] = 0x40000000
	}
	for i := range regs {
		if i%2 == 0 {
			oe := regs[i+1]
			dir := gpioMap.Read32(oe)
			masks[i] |= ^dir
		}
		addMonitor(gpioMap, regs[i], masks[i])
	}
	addMonitor(gpioMap, 0x270, 0)
	addMonitor(scuMap, 0x70, 0)
	addMonitor(scuMap, 0x8C, 0)
	for _, v := range []int64{0xc0, 0xc4, 0xc8, 0xcc, 0xd0, 0xd4, 0xd8, 0xdc} {
		addMonitor(scuMap, v, 0)
	}

	pmem.MonLoop(mons)
}

func usage() {
	log.Printf("usage: kpoke [options ]  <memaddr>.<spec>  [ <newval> ]\n")
	flag.PrintDefaults()
	os.Exit(1)
}

var cnt = 0

func i2cDec(changes []pmem.GpioChange) {
	a := make([]byte, len(changes)*4)
	for i, c := range changes {
		binary.LittleEndian.PutUint32(a[i*4:], c.New)
	}
	n, e := os.Stdout.Write(a)
	if n != len(a) || e != nil {
		log.Fatalf("write(%d)->%d, %s\n", len(a), n, e)
	}
	log.Printf("Write %d entries to stdout\n", len(changes))
}

func main() {
	var astGpio, astDump bool
	var astVga, mmapOp bool
	var imxDump bool
	var write, wflag bool
	var val int64
	var mon int
	var i2cmon int

	enc := binary.LittleEndian
	flag.BoolVar(&astGpio, "astgpio", false, "Special mon for ast gpio")
	flag.BoolVar(&astDump, "astdump", false, "Dump AST important regs")
	flag.BoolVar(&astVga, "astvga", false, "Use AST VGA device to access AST AS")
	flag.StringVar(&pmem.DevName, "devmem", "/dev/mem", "mem device to use")
	flag.BoolVar(&imxDump, "imxdump", false, "Dump ecspi2 imx registers")
	flag.IntVar(&mon, "mon", 0, "Monitor one register")
	flag.IntVar(&i2cmon, "i2cmon", -1, "Monitor a aspeed i2c bus")
	flag.BoolVar(&mmapOp, "mmap", true, "Use mmap (rather than pread/pwrite)")
	flag.BoolVar(&wflag, "w", false, "Write block (used with x.n")

	flag.Parse()

	if astVga {
		mapFn = ast.Map
	} else if mmapOp {
		mapFn = pmem.Map
	} else {
		mapFn = pmem.FileMap
	}
	if astDump {
		scuDump()
		gpioDump()
		lpcDump()
		return
	}
	if imxDump {
		imxMuxDump()
		return
	}
	if mon != 0 {
		page := int64(mon) & PgMask
		off := int64(mon) & (PgSize - 1)
		m := mapFn("reg", page, false, 0)
		pmem.MonLoop([]*pmem.Monitor{{M: m, Off: off, Mask: 0}})
	}
	if i2cmon >= 0 {
		a := ast.New()
		base := a.I2cBase(i2cmon)
		i2c := mapFn("i2c", 0x1e78a000, false, 0)
		log.Printf("monitoring i2c bus %d: 0x%08x\n", i2cmon, 0x1e78a000+base)
		pmem.MonLoopFun([]*pmem.Monitor{{M: i2c, Off: base + 0x14, Mask: ^uint32(0x60000)}}, i2cDec, nil)
	}
	if astGpio {
		astGpioMon()
		return
	}
	if len(flag.Args()) < 1 || len(flag.Args()) > 2 {
		usage()
	}
	locs := strings.Split(flag.Arg(0), ".")
	addr, err := strconv.ParseInt(locs[0], 0, 64)
	if err != nil {
		log.Fatalf("cannot parse %s\n", locs[0])
	}
	if len(flag.Args()) >= 2 {
		write = true
		val, _ = strconv.ParseInt(flag.Arg(1), 0, 64)
	}

	var mod byte
	var ioLen int64
	if len(locs) == 1 {
		mod = 'L'
	} else if len(locs[1]) == 1 {
		mod = locs[1][0]
	} else {
		ioLen, _ = strconv.ParseInt(locs[1], 0, 32)
		mod = 0
		write = wflag
	}

	page := addr & PgMask
	off := addr & (PgSize - 1)

	mapLen := int64(0)
	if off+ioLen > PgSize {
		mapLen = off + ioLen + (-off-ioLen)&^PgMask
	}
	data := mapFn("", page, write, mapLen)
	var res uint64
	if mod != 0 {
		f := "%#x"
		if write {
			f := "%#x"
			switch mod {
			case '8', 'q', 'Q':
				*(*uint64)(unsafe.Pointer(&data.Mem()[off])) = uint64(val)
			case '4', 'l', 'L':
				f = "%#08x"
				data.Write32(off, uint32(val))
			case '2', 'w', 'W':
				*(*uint16)(unsafe.Pointer(&data.Mem()[off])) = uint16(val)
			case '1', 'b', 'B':
				data.Write8(off, byte(val))
			}
			fmt.Printf("%#08x.%c := "+f+"\n", addr, mod, val)
		} else {
			switch mod {
			case '8', 'q', 'Q':
				res = *(*uint64)(unsafe.Pointer(&data.Mem()[off]))
			case '4', 'l', 'L':
				f = "%#08x"
				res = uint64(data.Read32(off))
			case '2', 'w', 'W':
				res = uint64(*(*uint16)(unsafe.Pointer(&data.Mem()[off])))
			case '1', 'b', 'B':
				res = uint64(data.Read8(off))
			}
			fmt.Printf("%#08x.%c = "+f+"\n", addr, mod, res)
		}
	} else {
		if write {
			b := make([]byte, ioLen)
			ReadFull(os.Stdin, b)
			for i := int64(0); i < ioLen; i += 4 {
				data.Write32(off+i, enc.Uint32(b[i:]))
			}
		} else {
			out := bufio.NewWriter(os.Stdout)
			for i := int64(0); i < ioLen; i += 4 {
				var b [4]byte
				binary.LittleEndian.PutUint32(b[:], data.Read32(off+i))
				_, err := out.Write(b[:])
				if err != nil {
					log.Fatal(err)
				}
			}
			err := out.Flush()
			if err != nil {
				log.Fatal(err)
			}
		}
	}
}
