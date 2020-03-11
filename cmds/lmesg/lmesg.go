package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
)

type Printk struct {
	Nsec     uint64
	RecLen   uint16
	TextLen  uint16
	DictLen  uint16
	Facility uint8
	Flags    uint8
}

func main() {
	in := flag.String("in", "", "file to parse")
	flag.Parse()

	fmt.Printf("hello lmesg\n")
	buf, err := ioutil.ReadFile(*in)
	if err != nil {
		log.Fatal(err)
	}
	hdrLen := uint16(binary.Size(Printk{}))
	for len(buf) >= int(hdrLen) {
		var p Printk
		err := binary.Read(bytes.NewReader(buf[0:hdrLen]), binary.LittleEndian, &p)
		if err != nil {
			log.Fatal(err)
		}

		if p.RecLen <= hdrLen {
			log.Fatalf("Rec invalid:%#v\n", p)
		}

		fmt.Printf("%6.6f: %s\n", float64(p.Nsec)*1e-9, string(buf[hdrLen:hdrLen+p.TextLen]))
		buf = buf[p.RecLen:]
	}

}
