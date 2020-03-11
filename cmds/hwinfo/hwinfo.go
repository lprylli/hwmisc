package main

import (
	"log"

	"github.com/lprylli/hwmisc/pci"
)

func main() {
	world := pci.PciInit()
	for _, d := range []int{10, 12} {
		imc := world.DevFn[pci.BDF(0x64, d, 0)]
		log.Printf("vid, did= %04x:%04x", imc.Read16(0), imc.Read16(2))
		if imc == nil {
			log.Fatal("Not a Xeon-SP proc\n")
		}
		for x := 0; x < 12; x++ {
			reg := imc.Read32(0x80 + x*4)
			limit := (reg >> 12) / 16
			log.Printf("TadLimit= 0x%08x , %d GB\n", reg, limit)
		}
		reg := imc.Read32(0x87c)
		log.Printf("cfg: 0x%08x\n", reg)
	}
}
