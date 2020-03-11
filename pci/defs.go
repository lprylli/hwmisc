package pci

import (
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"sort"
)

const (
	// Legacy pci header definitions
	PCI_STATUS             = 6
	PCI_STATUS_SERR        = 0x4000
	PCI_HEADER_TYPE        = 0xe
	PCI_HEADER_TYPE_NORMAL = 0
	PCI_HEADER_TYPE_BRIDGE = 1
	PCI_CAPABILITY_LIST    = 0x34

	// Pcie Capability
	PCI_CAP_ID_EXP = 0x10

	PCI_CAP_EXP_TYPE_ENDPOINT   = 0
	PCI_CAP_EXP_TYPE_ROOT_PORT  = 4
	PCI_CAP_EXP_TYPE_UPSTREAM   = 5
	PCI_CAP_EXP_TYPE_DOWNSTREAM = 6
	PCI_CAP_EXP_TYPE_PCI_BRIDGE = 7

	// PCIe AER Cap
	PCI_ECAP_ID_AER = 1
)

type World struct {
	Devs      []*PciDev
	DevFn     map[int]*PciDev
	Bus       map[int]*PciDev
	fd        *os.File
	nameCount map[string]int
}

type BasePciDev struct {
	w              *World
	Name           string
	parent         *PciDev
	LnkChild       *PciDev
	fd             *os.File
	Ecap           map[int]int
	Ocap           map[int]int
	devType        int
	secondary      int
	domain         int
	bus            int
	devFn          int
	reqId          int
	devClass       uint16
	headerType     int
	LnkSpeed       int
	LnkWidth       int
	LnkCapSpeed    int
	LnkCapWidth    int
	downPorts      int
	nickname, Path string
	vendor, device uint16
	App            interface{}
}

var le = binary.LittleEndian

func (d *PciDev) PciStatus() uint16 {
	return d.Read16(PCI_STATUS)
}
func (d *PciDev) EcapInit() {
	//log.Printf("%s:ecapInit", d.Name)

	d.Ecap = make(map[int]int)
	off := 0x100
	for off != 0 {
		cdef := d.Read32(off)
		if cdef == 0 || cdef == ^uint32(0) {
			break
		}
		// [ next-hdr: 12bit, ver: 4bit, ecap-id: 16bit ]
		ecap := cdef & 0xff
		//ver := (cdef >> 16) & 0xf
		d.Ecap[int(ecap)] = off
		//log.Printf("%s:ecap-id==%d at %#03x", d.Name, ecap, off)
		off = int(cdef >> 20)
	}
}

func (d *PciDev) capInit() {
	//log.Printf("%s:capInit", d.Name)
	d.Ocap = make(map[int]int)
	if d.PciStatus()&0x10 == 0 {
		return
	}
	start := int(d.Read8(PCI_CAPABILITY_LIST))
	for start >= 0x40 && start < 0xff {
		cdef := d.Read16(start)
		id := int(cdef & 0xff)
		if id == 0xff {
			break
		}
		d.Ocap[id] = start
		start = int(((cdef >> 8) & 0xfc))
	}
}

func (d *PciDev) GetSpeed() {
	lnkSta := d.Read16(d.Ocap[PCI_CAP_ID_EXP] + 0x12)
	d.LnkSpeed = int(lnkSta & 0xf)
	d.LnkWidth = int((lnkSta >> 4) & 0x3f)
}

func (w *World) AddDev(d *PciDev) {
	defer d.Uncache()
	if w.DevFn == nil {
		w.DevFn = make(map[int]*PciDev)
		w.Bus = make(map[int]*PciDev)
	}
	d.w = w
	d.reqId = d.bus*256 + d.devFn
	d.devType = -1
	d.vendor = d.Read16(0)
	d.device = d.Read16(2)
	if d.vendor == 0xffff && d.device == 0xffff {
		log.Printf("%s: ignored (0xffff on id)\n", d.Name)
		return
	}
	// Now commit to putting dev it in list.
	w.Devs = append(w.Devs, d)

	d.devClass = d.Read16(0xa)
	d.capInit()
	capExp := d.Ocap[PCI_CAP_ID_EXP]
	if capExp > 0 {
		d.devType = int((d.Read8(capExp+2) >> 4) & 0xf)
		lnkCap := d.Read16(capExp + 0xc)
		d.LnkCapWidth = int((lnkCap >> 4) & 0x3f)
		d.LnkCapSpeed = int(lnkCap & 0xf)
	}
	d.EcapInit()
	d.headerType = int(d.Read8(PCI_HEADER_TYPE)) & 0x7f
	if d.headerType == PCI_HEADER_TYPE_BRIDGE {
		d.secondary = int(d.Read8(0x19))
		w.Bus[d.secondary] = d
		//log.Printf("bus %#02x: mgt by %s", d.secondary, d.Name)
	}
	w.DevFn[d.reqId] = d
	d.Uncache()

}

func (d *PciDev) InitNickName() {
	separator := "/"
	var name string
	switch {
	case d.devType == PCI_CAP_EXP_TYPE_ROOT_PORT:
		name = "ROOT"
	case d.devType == PCI_CAP_EXP_TYPE_UPSTREAM && d.vendor == 0x8086:
		name = "ISW"
	case d.devType == PCI_CAP_EXP_TYPE_UPSTREAM && d.vendor == 0x10b5:
		name = "PLX"
	case d.devType == PCI_CAP_EXP_TYPE_UPSTREAM && d.vendor == 0x11f8:
		name = "RIMFIRE"
	case d.devType == PCI_CAP_EXP_TYPE_DOWNSTREAM:
		name = fmt.Sprintf("p%d", d.parent.downPorts)
		d.parent.downPorts++
		separator = ""
	case d.devType == PCI_CAP_EXP_TYPE_ENDPOINT && d.vendor == 0x15b7:
		name = "sandisk"
	case d.devType == PCI_CAP_EXP_TYPE_ENDPOINT && d.vendor == 0x15b3 && d.devFn == 0:
		name = "MLX"
	case d.devType == PCI_CAP_EXP_TYPE_PCI_BRIDGE && d.vendor == 0x1a03:
		name = "AST"
	case d.devType == PCI_CAP_EXP_TYPE_ENDPOINT && d.vendor == 0x8086 && d.devClass == 0x0200:
		name = "IETH"
	case d.devType == PCI_CAP_EXP_TYPE_ENDPOINT && d.vendor == 0x1425 && d.devClass == 0x0200 && d.devFn == 0:
		name = "CHL"
	case d.devType == PCI_CAP_EXP_TYPE_ENDPOINT && d.vendor == 0x8086 && d.devClass == 0x0106:
		name = "INTEL-SATA"
	case d.devType == PCI_CAP_EXP_TYPE_ENDPOINT && d.vendor == 0x8086 && d.devClass == 0x0107:
		name = "INTEL-SAS"
	case d.devType == PCI_CAP_EXP_TYPE_ENDPOINT && d.vendor == 0x1000 && d.devClass == 0x0107:
		name = "LSI-SAS"
	}
	if separator == "" {
		// downstream port of a switch
		d.Path = d.parent.Path + name
		d.nickname = d.parent.nickname + name
		return
	}

	if name == "" {
		if d.devFn == 0 {
			d.nickname = fmt.Sprintf("bus-%02xh", d.bus)
		} else if d.parent != nil && d.parent.LnkChild != nil {
			d.nickname = fmt.Sprintf("%s.%d", d.parent.LnkChild.nickname, d.devFn)
		} else {
			d.nickname = fmt.Sprintf("%02x:%02x.%d", d.bus, d.devFn/8, d.devFn%8)
		}
	} else {
		cnt := d.w.nameCount[name]
		d.w.nameCount[name] = cnt + 1
		d.nickname = fmt.Sprintf("%s%d", name, cnt)
	}
	if d.parent != nil {
		d.Path = d.parent.Path + separator + d.nickname
	} else {
		d.Path = d.nickname
	}
}

func (w *World) FindById(vid uint16, did uint16) (l []*PciDev) {
	for _, d := range w.Devs {
		if d.vendor == vid && d.device == did {
			l = append(l, d)
		}
	}
	return
}

func PciInit() *World {
	w := Scan()
	w.nameCount = make(map[string]int)
	sort.Slice(w.Devs, func(i, j int) bool { return w.Devs[i].Name < w.Devs[j].Name })
	for _, d := range w.Devs {
		upDev := w.Bus[d.bus]
		parentName := "none"
		if upDev != nil {
			parentName = upDev.Name
			d.parent = upDev
			if d.devFn == 0 && (upDev.devType == PCI_CAP_EXP_TYPE_ROOT_PORT || upDev.devType == PCI_CAP_EXP_TYPE_DOWNSTREAM) {
				upDev.LnkChild = d
			}
		}
		if d.devFn == 0 && false {
			log.Printf("dev %s on bus %#02x (p=%s)", d.Name, d.bus, parentName)
		}

	}
	for _, d := range w.Devs {
		d.InitNickName()
	}
	return w
}

func (d *PciDev) Read8(off int) uint8 {
	return d.Read(off, 1)[0]
}

func (d *PciDev) Read16(off int) uint16 {
	return le.Uint16(d.Read(off, 2))

}

func (d *PciDev) Read32(off int) uint32 {
	return le.Uint32(d.Read(off, 4))

}

func (d *PciDev) Write32(off int, data uint32) {
	var buf [4]byte
	le.PutUint32(buf[:], data)
	d.Write(off, buf[:])

}

func (d *PciDev) Write16(off int, data uint16) {
	var buf [2]byte
	le.PutUint16(buf[:], data)
	d.Write(off, buf[:])

}

func BDF(bus, dev, fn int) int {
	return 256*bus + dev*8 + fn
}
