package ast

import (
	"fmt"
	"log"
	"runtime"
	"time"

	"github.com/lprylli/hwmisc/pmem"
	"periph.io/x/periph/conn/i2c"
	"periph.io/x/periph/conn/physic"
)

var ExtraCheck bool

type i2cBus struct {
	m        pmem.Region
	base     int64
	bus      int
	bytes    int
	starting bool
}

func (i *i2cBus) String() string {
	return fmt.Sprintf("asti2c-%d", i.bus)
}

func (i *i2cBus) Tx(addr uint16, w, r []byte) error {
	if Verbose {
		fmt.Printf("slave:0x%02x w=%02x r%d\n", addr, w, len(r))
	}
	i.bytes = 0
	i.Start(false)
	if len(w) > 0 {
		err := i.TxByte(byte(addr * 2))
		if err != nil {
			i.Stop()
			return err
		}
		for _, b := range w {
			err = i.TxByte(b)
			if err != nil {
				i.Stop()
				return err
			}
		}

	}
	if len(r) > 0 {
		if len(w) > 0 {
			i.Start(true)
		}
		i.TxByte(byte(addr*2) + 1)
		for n, _ := range r {
			r[n] = i.RxByte(n == len(r)-1)
		}
	}
	i.Stop()
	return nil
}

func (i *i2cBus) SetSpeed(f physic.Frequency) error {
	return nil
}

func (a *AstHandle) I2cDev(busno, slave int) i2c.Dev {
	bus := &i2cBus{m: a.I2c, bus: busno, base: I2cBase(busno)}
	if bus.m.Read32(bus.base+0x14) != 0x0a060000 {
		bus.m.Write32(bus.base+0x14, 1<<11)
		time.Sleep(50 * time.Millisecond)
		bus.m.Write32(bus.base+0, 0)
		time.Sleep(100 * time.Millisecond)
	}
	if busno >= 2 && busno <= 13 {
		scuBit := uint32(1) << uint(busno+16-2)
		if a.scu.Read32(0x90)&scuBit == 0 {
			a.scu.Write32(0x90, a.scu.Read32(0x90)|scuBit)
			log.Printf("i2c %d pins were disabled, enabling...\n", busno)
		}
	}
	if bus.m.Read32(bus.base+0x14) != 0x0a060000 {
		log.Fatalf("Cannot initialize i2c, cmd = 0x%08x\n", bus.m.Read32(bus.base+0x14))
	}
	bus.m.Write32(bus.base, 1) // enable master func
	return i2c.Dev{Bus: bus, Addr: uint16(slave)}
}

func (i *i2cBus) Wait(op string) {
	t := time.Now()
	for cmd := i.m.Read32(i.base + 0x14); cmd&0x3ff != 0; cmd = i.m.Read32(i.base + 0x14) {
		runtime.Gosched()
		if time.Now().After(t.Add(time.Second)) {
			sts := i.m.Read32(i.base + 0x10)
			i.m.Write32(i.base+0, 0)
			log.Fatalf("Timeout on i2c bus during %s, cmd=0x%08x, sts=0x%08x", op, cmd, sts)
		}
	}
	time.Sleep(50 * time.Microsecond)
}

func (i *i2cBus) Start(repeat bool) {
	i.starting = true
	cmd := i.m.Read32(i.base + 0x14)
	if !(cmd == 0x0a060000 && !repeat) && !(cmd == 0x0c430000 && repeat) && !(repeat && !ExtraCheck) {
		log.Fatalf("%s: Start when non-idle: repeat=%t, st=0x%08x", i, repeat, cmd)
	}
	i.m.Write32(i.base+0x14, 1)
	i.Wait("start")
	sts := i.m.Read32(i.base + 0x10)
	if sts != 0 {
		log.Fatalf("Start: sts == 0x%08x\n", sts)
	}
}

func (i *i2cBus) TxByte(b byte) error {
	cmd := i.m.Read32(i.base + 0x14)
	if !(cmd == 0x14410000 && i.starting) && !(cmd == 0x0c430000 && !i.starting) && ExtraCheck {
		time.Sleep(100 * time.Microsecond)
		cmd2 := i.m.Read32(i.base + 0x14)
		log.Fatalf("%s: TxByte-%d with cmd=0x%08x 0x%08x 0x%08x", i, i.bytes, cmd, cmd2, i.m.Read32(i.base+0x10))
	}
	i.starting = false
	i.m.Write32(i.base+0x20, uint32(b))
	i.m.Write32(i.base+0x14, 2)
	i.Wait("txbyte")
	sts := i.m.Read32(i.base + 0x10)
	i.m.Write32(i.base+0x10, sts)
	if sts&2 != 0 {
		return fmt.Errorf("%s: TxByte-%02x (%d) was nacked (sts=0x%08x)\n", i, b, i.bytes, sts)
	}
	if sts != 1 {
		log.Fatalf("TxByte-%d: sts == 0x%08x\n", i.bytes, sts)
	}
	i.bytes += 1
	return nil
}

func (i *i2cBus) RxByte(last bool) byte {
	cmd := i.m.Read32(i.base + 0x14)
	if !(cmd&^0x20000 == 0x0c410000) && ExtraCheck {
		time.Sleep(200 * time.Microsecond)
		cmd2 := i.m.Read32(i.base + 0x14)
		log.Fatalf("%s: RxByte with cmd=0x%08x %08x %08x", i, cmd, cmd2, i.m.Read32(i.base+0x10))
	}
	if last {
		i.m.Write32(i.base+0x14, 0x18)
	} else {
		i.m.Write32(i.base+0x14, 8)
	}
	i.Wait("rxbyte")
	sts := i.m.Read32(i.base + 0x10)
	b := i.m.Read32(i.base + 0x20)
	//fmt.Printf("B%d:%04x\n", i.bytes, b)
	b = b >> 8
	if sts != 0x4 {
		log.Fatalf("RxByte: sts == 0x%08x\n", sts)
	}
	i.m.Write32(i.base+0x10, sts)
	i.bytes += 1
	return byte(b)
}

func (i *i2cBus) Stop() {
	cmd := i.m.Read32(i.base + 0x14)
	if !(cmd&^0x20000 == 0x0c410000) && ExtraCheck {
		log.Fatalf("%s: Stop with st=0x%08x", i, cmd)
	}
	i.m.Write32(i.base+0x14, 0x20)
	i.Wait("stop")
	sts := i.m.Read32(i.base + 0x10)
	if sts != 0x10 {
		log.Fatalf("stop: sts == 0x%08x\n", sts)
	}
	i.m.Write32(i.base+0x10, sts)
}
