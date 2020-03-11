package main

import (
	"fmt"

	"github.com/lprylli/hwmisc/ast"
)

func scuDump() {
	m := mapFn("scu", ast.SCU_ADDR, false, 4096)
	for r := int64(0); r <= 0x1dc; r += 4 {
		fmt.Printf("scu[%x]=0x%x\n", r, m.Read32(r))
	}
}

func gpioDump() {
	m := mapFn("gpio", ast.GPIO_ADDR, false, 4096)
	for r := int64(0); r <= 0x3a0; r += 4 {
		fmt.Printf("gpio[%x]=0x%x\n", r, m.Read32(r))
	}
}

func lpcDump() {
	m := mapFn("gpio", ast.LPC_ADDR, false, 4096)
	for r := int64(0); r <= 0x25c; r += 4 {
		fmt.Printf("lpc[%x]=0x%x\n", r, m.Read32(r))
	}
}

var iMxMuxs = []struct {
	Name   string
	Off    int64
	Expect int64
}{
	/* MISO */
	{"PAD_SD2_DATA03", 0x1f0, 3},
	{"PAD_I2C2_SDA", 0xc0, -8},

	/* mosi */
	{"PAD_SD2_DATA02", 0x1ec, 3},
	{"PAD_I2C2_SCL", 0xbc, -8},

	/* rdy */
	{"PAD_GPIO3_IO01", 0x108, 8},

	/* scl */
	{"PAD_SD2_DATA00", 0x1e4, 3},
	{"PAD_UART4_TX_DATA", 0x340, -8},

	/* ss0 */
	{"PAD_SD2_DATA01", 0x1e8, 3},
	{"UART4_RX_DATA", 0x344, -8},

	/* ss1*/
	{"PAD_GPIO3_IO02", 0x10c, 8},

	/* ss2*/
	{"PAD_GPIO3_IO03", 0x110, 8},

	/* ss3*/
	{"PAD_GPIO3_IO04", 0x114, 8},
}

var iMxDaisy = []struct {
	Name   string
	Off    int64
	Expect int64
}{
	{"ECSPI2_SCLK", 0x544, 0},
	{"ECSPI2_MISO", 0x548, 0},
	{"ECSPI2_MOSI", 0x54c, 1},
	{"ECSPI2_SS0", 0x550, 0},
}

func match(val32 uint32, expect int64) string {
	val := int64(val32)
	switch {
	case (expect >= 0 && val == expect) || (expect < 0 && val != expect):
		return fmt.Sprintf("ok(%d)", val)
	case expect < 0:
		return fmt.Sprintf("expect != %d but got %d", -expect, val)
	default:
		return fmt.Sprintf("expect = %d but got %d", expect, val)
	}
}

func imxMuxDump() {
	m := mapFn("imx.pinmux", 0x20E0000, false, 0)

	for _, mux := range iMxMuxs {
		reg := m.Read32(mux.Off)
		fmt.Printf("%s: force=%d mode=ALT%d %s\n", mux.Name, (reg>>4)&1, reg&0xf, match(reg&0xf, mux.Expect))
	}
	for _, daisy := range iMxDaisy {
		reg := m.Read32(daisy.Off)
		fmt.Printf("%s: input=%d %s\n", daisy.Name, reg&1, match(reg&1, daisy.Expect))
	}
}
