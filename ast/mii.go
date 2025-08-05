package ast

import (
	"fmt"
	"time"
)

func (a *AstHandle) MiiInfo() {
	if a.Family == 25 {
		mac := a.mac[1]
		rev := mac.Read32(0x40)
		oldMdio := rev&0x8000_0000 == 0
		if oldMdio {
			cmd := mac.Read32(0x60)
			for try := 0; try < 2; try++ {
				if cmd&0xffff0000 == 0x0020_0000 {
					status := mac.Read32(0x64)
					if status&0xff00_0000 == 0x7900_0000 {
						fmt.Printf("link-up: %d\n", (status>>18)&1)
						return
					}
				}
				if cmd&0xfc00_0000 == 0 {
					mac.Write32(0x60, 0x0420_000f)
					time.Sleep(time.Millisecond)
					cmd = mac.Read32(0x60)
				} else {
					break
				}
			}
			fmt.Printf("Cannot get link status (mii busy [0x60]=0x%08x)\n", cmd)
		} else {
			cmd := mac.Read32(0x60)
			for try := 0; try < 2; try++ {
				if cmd == 0x1801 {
					status := mac.Read32(0x64)
					if status&0xff00 == 0x7900 {
						fmt.Printf("link-up: %d\n", (status>>2)&1)
						return
					}
				}
				if cmd&0xffff8000 == 0 {
					mac.Write32(0x60, 0x9801)
					time.Sleep(time.Millisecond)
					cmd = mac.Read32(0x60)
				} else {
					break
				}
			}
			fmt.Printf("Cannot get link status (mii busy [0x60]=0x%08x)\n", cmd)
		}
	} else if a.Family == 26 {
		pmi := Map("PMI", 0x1e650000, true, 4096)
		cmd := pmi.Read32(0x00)
		for try := 0; try < 2; try++ {
			if cmd == 0x18010000 {
				status := pmi.Read32(0x4)
				if status&0xff00 == 0x7900 {
					fmt.Printf("link-up: %d\n", (status>>2)&1)
					return
				}
			}
			if cmd&0x8000ffff == 0 {
				pmi.Write32(0x0, 0x98010000)
				time.Sleep(time.Millisecond)
				cmd = pmi.Read32(0x0)
			} else {
				break
			}
		}
		fmt.Printf("Cannot get link status (mii busy [0x0]=0x%08x)\n", cmd)
	} else {
		fmt.Printf("mii status not supported on %d00", a.Family)
	}
}
