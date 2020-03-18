package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"io/ioutil"
	"log"

	"github.com/lprylli/hwmisc/ast"
	"periph.io/x/periph/conn/i2c"
)

const (
	BCVSTS = 0xc
)

func BIT(val uint32, bit uint32) uint32 {
	return (val >> bit) & 1
}

func idt_eeread(dev i2c.Dev, addr uint32) byte {
	tx := []byte{0x47, 4, 1, 0, byte(addr % 256), byte(addr / 256)}
	err := dev.Tx(tx, nil)
	if err != nil {
		log.Fatalf("idt_eeread:query-phase:%s\n", err)
	}
	var rbuf [8]byte
	err = dev.Tx([]byte{0x47}, rbuf[:])
	if err != nil {
		err = dev.Tx([]byte{0x47}, rbuf[:])
	}
	if err != nil {
		log.Fatalf("idt_eeread:read-phase:%s\n", err)
	}
	if rbuf[0] != 5 || rbuf[1] != tx[2] || rbuf[2] != tx[3] || rbuf[3] != tx[4] || rbuf[4] != tx[5] {
		log.Fatalf("idt_eeread:invalid-recv-data:got: %02x", rbuf)
	}
	return rbuf[5]
}

func idt_eewrite(dev i2c.Dev, addr uint32, b byte) {
	tx := []byte{0x47, 5, 0, 0, byte(addr % 256), byte(addr / 256), b}
	err := dev.Tx(tx, nil)
	if err != nil {
		log.Fatalf("idt_eewrite:%s\n", err)
	}
}

func idt_reg(dev i2c.Dev, reg uint32) uint32 {
	regI2c := reg / 4
	tx := []byte{0x43, 3, 0x1f, byte(regI2c % 256), byte(regI2c / 256)}
	err := dev.Tx(tx, nil)
	if err != nil {
		log.Fatalf("idt_reg:query-phase:%d\n", err)
	}
	var rbuf [8]byte
	err = dev.Tx([]byte{0x43}, rbuf[:])
	if err != nil {
		log.Fatalf("idt_reg:query-phase:%d\n", err)
	}
	if rbuf[0] != 7 || rbuf[1] != tx[2] || rbuf[2] != tx[3] || rbuf[3] != tx[4] {
		log.Fatalf("idt_reg:recv:got: %02x", rbuf)
	}
	return binary.LittleEndian.Uint32(rbuf[4:8])
}

func idtInfo(dev i2c.Dev) {
	id := idt_reg(dev, 0)
	fmt.Printf("IDT id= 0x%08x\n", id)
	bcvsts := idt_reg(dev, BCVSTS)
	var d string
	var merge uint32
	if id == 0x80e4111d {
		merge := BIT(bcvsts, 27)
		d = "x4 x4"
		if merge != 0 {
			d = "x8"
		}
	} else if id == 0x80e0111d {
		merge := (bcvsts >> 27) & 0x7
		switch merge {
		case 0:
			d = "x4 x4 x4 x4"
		case 1:
			d = "x4 x4 x8"
		case 2:
			d = "x8 x4 x4"
		case 3:
			d = "x8 x8"
		default:
			d = "x16"
		}
	} else {
		log.Fatalf("Unknow chip: 0x%08x", id)
	}
	fmt.Printf("MERGE = %d: %s\n", merge, d)

}

func idtRead(dev i2c.Dev) []byte {
	b := make([]byte, 1024)
	for i := uint32(0); int(i) < len(b); i++ {
		b[i] = idt_eeread(dev, i)
	}
	return b
}

func idtWrite(dev i2c.Dev, b []byte) {
	for i, c := range b {
		idt_eewrite(dev, uint32(i), c)
	}
}

func Tx(dev i2c.Dev, w, r []byte) {
	err := dev.Tx(w, r)
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	var info bool
	var eein, eeout string
	var reg int64
	var mux70 int
	flag.BoolVar(&ast.Verbose, "verbose", false, "verbosity")
	flag.BoolVar(&info, "info", false, "info about IDT chip")
	flag.Int64Var(&reg, "reg", -1, "read one IDT register")
	flag.StringVar(&eein, "eein", "", "file to program to IDT ee")
	flag.StringVar(&eeout, "eeout", "", "file where to store current IDT ee contents")
	flag.IntVar(&mux70, "mux70", 8, "pca954x(0x70) setting to access desired pcie-slot smbus")
	flag.BoolVar(&ast.ExtraCheck, "devcheck", true, "enable dev sanity checks")

	flag.Parse()
	if reg == -1 && eein == "" && eeout == "" {
		info = true
	}

	ast := ast.New()
	ast.AstStop()
	dev := ast.I2cDev(2, 0x72)
	mux := ast.I2cDev(2, 0x70)
	var b [1]byte
	Tx(mux, nil, b[:])

	fmt.Printf("mux70 current setting:0x%02x\n", b[0])
	mux70b := byte(mux70)
	if b[0] != mux70b {
		Tx(mux, []byte{mux70b}, nil)
		Tx(mux, nil, b[:])
		fmt.Printf("mux70 new setting:0x%02x\n", b[0])
	}
	if info {
		idtInfo(dev)
	}
	if eeout != "" {
		b := idtRead(dev)
		err := ioutil.WriteFile(eeout, b, 0666)
		if err != nil {
			log.Fatal(err)
		}
	}
	if eein != "" {
		buf, err := ioutil.ReadFile(eein)
		if err != nil {
			log.Fatal(err)
		}
		idtWrite(dev, buf)
	}
}
