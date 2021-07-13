package main

import (
	"debug/elf"
	"encoding/binary"
	"flag"
	"io/ioutil"
	"log"
	"os"
)

func main() {
	log.Printf("hello elfcode")
	in := flag.String("in", "", "data file")
	flag.Parse()
	f, _ := os.OpenFile("a.out", os.O_WRONLY | os.O_CREATE, 0666)
	data, _ := ioutil.ReadFile(*in)

	e := elf.Header32{}
	p := elf.Prog32{}

	copy(e.Ident[:],elf.ELFMAG)
	e.Ident[elf.EI_CLASS] = byte(elf.ELFCLASS32)
	e.Ident[elf.EI_DATA] = byte(elf.ELFDATA2LSB)
	e.Ident[elf.EI_VERSION] = byte(elf.EV_CURRENT)
	e.Ident[elf.EI_OSABI] = byte(elf.ELFOSABI_ARM)
	e.Type = uint16(elf.ET_CORE)
	e.Machine = uint16(elf.EM_ARM)
	e.Version = uint32(elf.EV_CURRENT)
	e.Phoff = 0
	e.Flags = 0
	e.Ehsize = uint16(binary.Size(e))
	e.Phentsize = uint16(binary.Size(p))
	e.Phnum = 1
	e.Phoff = uint32(e.Ehsize)
	binary.Write(f, binary.LittleEndian, e)
	p.Type = uint32(elf.PT_LOAD)
	p.Off = e.Phoff + uint32(e.Phentsize) * uint32(e.Phnum)
	p.Vaddr = 0x80000000
	p.Paddr = 0
	p.Filesz = uint32(len(data))
	p.Memsz = p.Filesz
	p.Flags = uint32(elf.PF_R | elf.PF_W | elf.PF_X)
	p.Align = 0
	binary.Write(f, binary.LittleEndian, p)
	f.Write(data)
	f.Close()
}
/*


elf->e_ident[EI_DATA] = ELF_DATA;
elf->e_ident[EI_VERSION] = EV_CURRENT;

	elf->e_ident[EI_OSABI] = ELF_OSABI;

	elf->e_type = ET_CORE;
	elf->e_machine = machine;
	elf->e_version = EV_CURRENT;
	elf->e_phoff = sizeof(struct elfhdr);
	elf->e_flags = flags;
	elf->e_ehsize = sizeof(struct elfhdr);
	elf->e_phentsize = sizeof(struct elf_phdr);
	elf->e_phnum = segs;}
*/

