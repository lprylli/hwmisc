package ast

import (
	"log"
	"strconv"
	"strings"
)

type Reg struct {
	Name   string
	Off    int64
	Fields []*RegField
}
type RegField struct {
	Parent            *Reg
	Name              string
	FirstBit, NumBits uint8
}

func ParseField(r *Reg, fields []string) {
	var f RegField
	var l = strings.Join(fields, " ")
	r.Fields = append(r.Fields, &f)
	f.Parent = r
	if len(fields) < 2 || fields[0] == "" || fields[1] == "" {
		log.Fatalf("Cannot parse field line: \"%s\"\n", l)
	}
	f.Name = strings.Join(fields[1:], " ")
	bits := strings.Split(fields[0], ":")
	b0, err := strconv.ParseInt(bits[0], 10, 8)
	if err != nil {
		log.Fatalf("Cannot parse field line: \"%s\"\n", l)
	}
	switch len(bits) {
	case 1:
		f.NumBits = 1
		f.FirstBit = uint8(b0)
	case 2:
		b1, err := strconv.ParseInt(bits[1], 10, 8)
		if err != nil || b0 <= b1 {
			log.Fatalf("Cannot parse field line: \"%s\"\n", l)
		}
		f.NumBits = uint8(b0 + 1 - b1)
		f.FirstBit = uint8(b1)
	default:
		log.Fatalf("Cannot parse field line: \"%s\"\n", l)
	}
}

func ParseReg(fields []string) *Reg {
	var r Reg
	if len(fields) < 2 || fields[0] == "" || fields[1] == "" {
		log.Fatalf("Cannot parse reg line: \"%s\"\n", strings.Join(fields, " "))
	}
	r.Name = strings.Join(fields[1:], " ")
	off, err := strconv.ParseInt(fields[0], 0, 32)
	r.Off = int64(off)
	if err != nil {
		log.Fatalf("Cannot int parse for reg line: \"%s %s\"\n", fields[0], r.Name)
	}
	return &r
}

func ParseRegs(def string) []*Reg {
	lines := strings.Split(def, "\n")
	var r *Reg
	var rList []*Reg
	for _, l := range lines {
		if l == "" {
			continue
		}
		fields := strings.Split(l, " ")
		if fields[0] == "" && fields[1] == "" {
			ParseField(r, fields[2:])
		} else {
			r = ParseReg(fields)
			rList = append(rList, r)
		}
	}
	return rList
}
