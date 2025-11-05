package pci

import (
	"syscall"
)

var _ syscall.Errno

type _Ctype___uint16_t = _Ctype_ushort

type _Ctype___uint32_t = uint32

type _Ctype___uint8_t = _Ctype_uchar

type _Ctype_char int8

type _Ctype_int int32

type _Ctype_pci_getconf_flags uint32

type _Ctype_pci_getconf_status uint32

type struct_pci_conf14 struct {
	pc_sel       struct_pcisel
	pc_hdr       _Ctype_u_int8_t
	pc_subvendor _Ctype_u_int16_t
	pc_subdevice _Ctype_u_int16_t
	pc_vendor    _Ctype_u_int16_t
	pc_device    _Ctype_u_int16_t
	pc_class     _Ctype_u_int8_t
	pc_subclass  _Ctype_u_int8_t
	pc_progif    _Ctype_u_int8_t
	pc_revid     _Ctype_u_int8_t
	pd_name      [17]_Ctype_char
	pd_unit      _Ctype_u_long
}

type struct_pci_conf struct {
	pc_sel          struct_pcisel
	pc_hdr          _Ctype_u_int8_t
	pc_subvendor    _Ctype_u_int16_t
	pc_subdevice    _Ctype_u_int16_t
	pc_vendor       _Ctype_u_int16_t
	pc_device       _Ctype_u_int16_t
	pc_class        _Ctype_u_int8_t
	pc_subclass     _Ctype_u_int8_t
	pc_progif       _Ctype_u_int8_t
	pc_revid        _Ctype_u_int8_t
	pd_name         [17]_Ctype_char
	pd_unit         _Ctype_u_long
	pd_numa_domain  int32
	pc_reported_len uint64
	pc_spare        [64]int8
}

type struct_pci_conf_io struct {
	pat_buf_len   _Ctype_u_int32_t
	num_patterns  _Ctype_u_int32_t
	patterns      *struct_pci_match_conf
	match_buf_len _Ctype_u_int32_t
	num_matches   _Ctype_u_int32_t
	matches       *struct_pci_conf
	offset        _Ctype_u_int32_t
	generation    _Ctype_u_int32_t
	status        _Ctype_pci_getconf_status
	_             [4]byte
}

type struct_pci_conf_io14 struct {
	pat_buf_len   _Ctype_u_int32_t
	num_patterns  _Ctype_u_int32_t
	patterns      *struct_pci_match_conf
	match_buf_len _Ctype_u_int32_t
	num_matches   _Ctype_u_int32_t
	matches       *struct_pci_conf14
	offset        _Ctype_u_int32_t
	generation    _Ctype_u_int32_t
	status        _Ctype_pci_getconf_status
	_             [4]byte
}

type struct_pci_io struct {
	pi_sel   struct_pcisel
	pi_reg   int32
	pi_width int32
	pi_data  _Ctype_u_int32_t
}

type struct_pci_match_conf struct {
	pc_sel    struct_pcisel
	pd_name   [17]_Ctype_char
	pd_unit   _Ctype_u_long
	pc_vendor _Ctype_u_int16_t
	pc_device _Ctype_u_int16_t
	pc_class  _Ctype_u_int8_t
	flags     _Ctype_pci_getconf_flags
	_         [4]byte
}

type struct_pcisel struct {
	pc_domain _Ctype_u_int32_t
	pc_bus    _Ctype_u_int8_t
	pc_dev    _Ctype_u_int8_t
	pc_func   _Ctype_u_int8_t
	_         [1]byte
}

type _Ctype_u_int16_t = _Ctype___uint16_t

type _Ctype_u_int32_t = _Ctype___uint32_t

type _Ctype_u_int8_t = _Ctype___uint8_t

type _Ctype_u_long = _Ctype_ulong

type _Ctype_uchar uint8

type _Ctype_ulong uint64

type _Ctype_ushort uint16

type _Ctype_void [0]byte

const PCIOCGETCONF_14 = 0xc0307005
const PCIOCGETCONF = 0xc030700a
const PCIOCREAD = 0xc0147002
const PCIOCWRITE = 0xc0147003
const PCI_GETCONF_LAST_DEVICE = 0x0
