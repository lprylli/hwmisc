package pci

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strconv"
)

const pciDir = "/sys/bus/pci/devices"

type PciDev struct {
	BasePciDev
}

func Scan() *World {
	var w World
	var re = regexp.MustCompile("^([0-9a-z]+):([0-9a-z]+):([0-9a-z]+)\\.([0-9a-z])$")

	files, err := ioutil.ReadDir(pciDir)
	if err != nil {
		log.Fatalln(err)
	}
	for _, f := range files {
		m, err := os.Stat(pciDir + "/" + f.Name() + "/config")

		if err != nil {
			log.Fatalln(err)
		}
		//fmt.Printf("%s:%t\n", f.Name(), m.Mode().IsRegular())
		if m.Mode().IsRegular() {
			d := PciDev{}
			d.Name = f.Name()
			r := re.FindStringSubmatch(d.Name)
			domain, _ := strconv.ParseInt(r[1], 16, 16)
			bus, _ := strconv.ParseInt(r[2], 16, 16)
			dev, _ := strconv.ParseInt(r[3], 16, 16)
			fn, _ := strconv.ParseInt(r[4], 16, 16)
			d.domain = int(domain)
			d.bus = int(bus)
			d.devFn = int(dev*8 + fn)
			s := fmt.Sprintf("%04x:%02x:%02x.%x", d.domain, d.bus, d.devFn/8, d.devFn%8)
			if s != d.Name {
				log.Fatalf("Error with %s (%02x:%02x) != %s", d.Name, d.bus, d.devFn, s)
			}
			w.AddDev(&d)
		}
	}
	return &w
}

func (d *PciDev) Read(off int, len int) []byte {
	var err error
	if d.fd == nil {
		d.fd, err = os.OpenFile(pciDir+"/"+d.Name+"/config", os.O_RDWR, 0666)
		if err != nil {
			log.Fatalln(err)
		}
	}
	buf := make([]byte, len)
	n, err := d.fd.ReadAt(buf, int64(off))
	if err != nil || n != len {
		if !(n == 0 && off == 256 && err == io.EOF) {
			log.Fatalf("Read(%d, %d)->(%d, %s)\n", off, len, n, err)
		}
	}
	return buf
}

func (d *PciDev) Write(off int, data []byte) {
	var err error
	if d.fd == nil {
		d.fd, err = os.OpenFile(pciDir+"/"+d.Name+"/config", os.O_RDWR, 0666)
		if err != nil {
			log.Fatalln(err)
		}
	}
	n, err := d.fd.WriteAt(data, int64(off))
	if err != nil || n != len(data) {
		log.Fatalf("%s:Write(%d, %d)->(%d, %s)\n", d.Name, off, len(data), n, err)
	}
}

func (d *PciDev) Uncache() {
	fd := d.fd
	d.fd = nil
	fd.Close()
}
