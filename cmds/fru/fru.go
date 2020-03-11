package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"reflect"
	"regexp"
	"strconv"
)

type Hdr struct {
	Version  uint8
	Internal uint8
	Chassis  uint8
	Board    uint8
	Product  uint8
	Multirec uint8
	Pad      uint8
	Checksum uint8 //  Header checksum
}

type Chassis struct {
	Part, Serial string
}

type Board struct {
	Mfg, Name, Sn, Part string
}

type Product struct {
	Mfg, Name, Part, Version, Serial, Tag string
}

func tlv(bufp *[]byte) string {
	buf := *bufp
	b := buf[0]
	if b&0xc0 != 0xc0 {
		log.Printf("Not ascii code: 0x%x\n", b)
	}
	n := b & 0x3f
	*bufp = buf[1+n:]
	return string(buf[1 : 1+n])
}

func Area(x interface{}, buf []byte, off int) {
	//fmt.Printf("vers=0x%x, len=%d, type=%d, %#x\n",  buf[0], buf[1], buf[2], buf[3])
	v := reflect.ValueOf(x).Elem()
	numFields := v.NumField()
	buf = buf[off:]
	for i := 0; i < numFields; i++ {
		f := v.Field(i)
		f.SetString(tlv(&buf))
	}
	fmt.Printf("%#v\n", v)
}

func main() {
	fName := flag.String("in", "", "File to parse")
	outName := flag.String("out", "", "File to write")
	ascii := flag.Bool("ascii", false, "File is in ascii format")
	flag.Parse()

	fmt.Printf("Reading fru from: \"%s\"\n", *fName)
	buf, err := ioutil.ReadFile(*fName)
	if err != nil {
		log.Fatal(err)
	}
	if *ascii {
		w := regexp.MustCompile("[ \n][ \n]*")
		vals := w.Split(string(buf), -1)
		fmt.Printf("number of vals = %d\n", len(vals))
		var b []byte
		for _, v := range vals {
			if v == "" {
				continue
			}
			c, e := strconv.ParseUint(v, 16, 8)
			if e != nil {
				log.Fatalf("byte %s: %s", v, e)
			}
			b = append(b, byte(c))
		}
		buf = b
	}
	if *outName != "" {
		e := ioutil.WriteFile(*outName, buf, 0666)
		if e != nil {
			panic(e)
		}
	}
	for i, x := range buf {
		fmt.Printf("%02x ", x)
		if i%16 == 15 {
			fmt.Printf("\n")
		}
	}
	var hdr Hdr
	if len(buf) <= binary.Size(hdr) {
		log.Fatalf("%s: File too small (%d) for fru\n", *fName, len(buf))
	}
	binary.Read(bytes.NewReader(buf), binary.LittleEndian, &hdr)
	fmt.Printf("Hdr=%#v\n", hdr)

	if hdr.Chassis != 0 {
		Area(&Chassis{}, buf[hdr.Chassis*8:], 3)
	}
	if hdr.Board != 0 {
		Area(&Board{}, buf[hdr.Board*8:], 6)
	}
	if hdr.Product != 0 {
		Area(&Product{}, buf[hdr.Product*8:], 3)
	}
}
