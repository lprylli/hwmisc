package ast

import (
	"log"
	"strings"
	"syscall"
)

func arch() string {
	uts := &syscall.Utsname{}
	if err := syscall.Uname(uts); err != nil {
		log.Fatal(err)
	}
	b := make([]byte, len(uts.Machine))
	for i, v := range uts.Machine {
		b[i] = byte(v)
	}
	str := string(b)
	str = strings.Split(str,"\x00")[0]
	return str
}
