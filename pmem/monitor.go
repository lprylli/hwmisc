package pmem

import (
	"fmt"
	"time"
)

type Monitor struct {
	M        Region
	Off      int64
	Mask     uint32
	OldVal   uint32
	OldValid bool
	Aux      interface{}
}

type GpioChange struct {
	Time  uint32
	Bank  string
	Addr  int64
	Old   uint32
	New   uint32
	Delay uint32
	Mask  uint32
	Aux   [3]uint32
	Mon   *Monitor
}

func GpioRes(c []GpioChange) {
	for _, v := range c {
		fmt.Printf("%6d %v %#08x oe=%#x\n", v.Delay, v, v.Old^v.New, ^v.Mask)
	}
}

func MonLoop(mons []*Monitor) {
	MonLoopFun(mons, GpioRes, nil)
}

func MonLoopFun(mons []*Monitor, batchfn func([]GpioChange), cfn func(c *GpioChange)) {
	change := make([]GpioChange, 0, 100)
	start := time.Now()
	lastChangeTime := uint32(0)
	iters := 0
	loop := func() {
		for {
			t := uint32(int64(time.Since(start)) >> 10)
			for _, m := range mons {

				val := m.M.Read32(m.Off)
				if m.OldValid && val&^m.Mask != m.OldVal&^m.Mask {
					c := GpioChange{Time: t, Bank: m.M.Name(), Addr: m.Off, Old: m.OldVal, New: val, Mask: m.Mask, Delay: t - lastChangeTime, Mon: m}
					change = append(change, c)
					lastChangeTime = t
					if cfn != nil {
						cfn(&change[len(change)-1])
					}

				}
				m.OldValid = true
				m.OldVal = val
			}
			iters++
			if len(change) >= 200 || (len(change) > 0 && t-change[0].Time >= 5000000) {
				//log.Printf("iters=%d\n", iters)
				iters = 0
				batchfn(change)
				change = change[0:0]
			}
		}
	}
	loop()
}
