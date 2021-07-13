package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/lprylli/hwmisc/pci"
)

type PciDev struct {
	*pci.PciDev
	errs     int64
	corrErrs [32]int64
	uncErrs  [32]int64
}

func (d *PciDev) Child() *PciDev {
	return d.LnkChild.App.(*PciDev)
}

//var devTypes = []string{"Endpoint", "Legacy", "Unknown-2", "Unknown-3", "Root-Port", "Upstream", "DownStream"}

var aerCorrErrDesc = map[int]string{
	0: "rcv-error", 6: "bad-tlp", 7: "bad-dllp",
	8: "replay-rollover", 12: "replay-timer", 13: "advisory-nf",
	15: "log-overflow"}

var aerUncErrDesc = map[int]string{
	4: "data-link-error", 5: "surprise-down", 12: "poisoned-tlp",
	13: "flow-ctl", 14: "cpl-timeout", 15: "cpl-abort",
	16: "unexp-cpl", 17: "rcv-overflow", 18: "malformed-tlp", 20: "UR", 21: "ACS-err"}

var pciExpErrDesc = map[int]string{
	0: "corr", 1: "nonFatal", 2: "Fatal", 3: "UR",
}

var aerUncUrMask uint32 = 1 << 20                // mask UR
var aerCorrUrMask uint32 = (1 << 13) | (1 << 15) // mask nf-advisory + log-overflow (set by UR evt).
var devExpErrMask uint16 = 0x9                   // mask UR+corr
var verbose bool

func errPoll(d *PciDev) (errors int64) {
	// correctable errors
	err := d.Read32(d.Ecap[pci.PCI_ECAP_ID_AER] + 0x10)
	err &^= aerCorrUrMask
	if err != 0 {
		d.Write32(d.Ecap[pci.PCI_ECAP_ID_AER]+0x10, err)
		//log.Printf("%s:cerr=#%08x", d.name, err)
		for i := 0; err != 0; i++ {
			if err&1 != 0 {
				d.corrErrs[i] += 1
				d.errs += 1
				errors += 1
			}
			err >>= 1
		}
	}
	// uncorrectable errors
	err = d.Read32(d.Ecap[pci.PCI_ECAP_ID_AER] + 0x4)
	err &^= aerUncUrMask
	if err != 0 {
		d.Write32(d.Ecap[pci.PCI_ECAP_ID_AER]+0x4, err)
		//log.Printf("%s:correrr=#%08x", d.name, err)
		for i := 0; err != 0; i++ {
			if err&1 != 0 {
				d.uncErrs[i] += 1
				d.errs += 1
				errors += 1
			}
			err >>= 1
		}
	}
	return errors
}

func statsGen(duration time.Duration, errs []int64, errMap map[int]string, errType string) {
	for i := 0; i < 32; i++ {
		if errs[i] > 0 {
			errDesc := errMap[i]
			if errDesc == "" {
				errDesc = fmt.Sprintf("%s-%d", errType, i)
			}
			fmt.Printf("    %s, count=%d: rate=%g err/s\n",
				errDesc, errs[i], float64(errs[i])/float64(duration)*1e9)
		}
	}
}
func stats(d *PciDev, duration time.Duration) {
	fmt.Printf("%s:\n", d.Path)
	statsGen(duration, d.corrErrs[:], aerCorrErrDesc, "corr")
	statsGen(duration, d.uncErrs[:], aerUncErrDesc, "unc")
	fmt.Printf("\n\n")

}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func showLinks(world *pci.World, aer bool) []*PciDev {
	var links []*PciDev
	for _, d := range world.Devs {
		if d.LnkChild != nil && verbose {
			log.Printf("%s:%d:%d\n", d.Name, d.Ecap[pci.PCI_ECAP_ID_AER], d.LnkChild.Ecap[pci.PCI_ECAP_ID_AER])
		}
		// Exception for some AMD GPP root port not advertising AER (1022:1483), and blacklist internal pcie links (1022:1484)
		upQual := (d.Ecap[pci.PCI_ECAP_ID_AER] > 0 && (d.Vendor != 0x1022 || d.Device != 0x1484)) || (!aer && d.Vendor == 0x1022 && d.Device == 0x1483)
		if d.LnkChild != nil && upQual && d.LnkChild.Ecap[pci.PCI_ECAP_ID_AER] > 0 {
			d.GetSpeed()
			c := d.LnkChild
			c.GetSpeed()
			var other string
			var buggy bool
			if c.LnkWidth != d.LnkWidth || c.LnkSpeed != d.LnkSpeed {
				other += fmt.Sprintf("down=x%d.gen%d ", c.LnkWidth, c.LnkSpeed)
				buggy = true
			}
			buggy = buggy || d.LnkWidth != min(d.LnkCapWidth, c.LnkCapWidth) || d.LnkSpeed != min(d.LnkCapSpeed, c.LnkCapSpeed)
			if buggy || verbose {
				other += fmt.Sprintf("upcap=x%d.gen%d ", d.LnkCapWidth, d.LnkCapSpeed)
				other += fmt.Sprintf("downcap=x%d.gen%d", c.LnkCapWidth, c.LnkCapSpeed)
			}
			fmt.Printf("%s (%s <-> %s)  (x%d.gen%d) %s\n", c.Path, d.Name, c.Name, d.LnkWidth, d.LnkSpeed, other)
			appDev := PciDev{PciDev: d}
			d.App = &appDev
			d.LnkChild.App = &PciDev{PciDev: d.LnkChild}
			links = append(links, &appDev)
		}
	}
	return links
}

func monLinks(world *pci.World, nbIter int, delayNano time.Duration) {
	links := showLinks(world, true)
	var totalErrors int64
	// The LinksWithErr map records whether any error was detected for a device
	// in the previous iteration (to decide whether to include it in next high-frequency poll phase)

	var prevLinksWithErr = make(map[*PciDev]bool)
	log.Printf("Monitoring %d links\n", len(links))

	// Each iteration last about one second, and has two phases:
	//  first we poll every link
	//  then we poll every link that has recently seen error at higher frequency.
	globalStart := time.Now()
	for i := 0; i < nbIter || nbIter == -1; i++ {
		//
		var errors int64
		var linksWithErr []*PciDev

		// go over all links to check for errors
		for _, d := range links {
			lnkErr := errPoll(d)
			lnkErr += errPoll(d.Child())
			errors += lnkErr
			if lnkErr > 0 || prevLinksWithErr[d] {
				linksWithErr = append(linksWithErr, d)
				prevLinksWithErr[d] = lnkErr > 0
			}
			prevLinksWithErr[d] = lnkErr > 0
		}
		start := time.Now()
		if len(linksWithErr) > 0 {
			// poll link having seen errors in higher frequency loop
			for !time.Now().After(start.Add(time.Second)) {
				for _, d := range linksWithErr {
					lnkErr := errPoll(d)
					lnkErr += errPoll(d.Child())
					errors += lnkErr
					if lnkErr > 0 { // in high-freq loop we purposefully not reset prevLinksWithErr to false
						prevLinksWithErr[d] = true
					}
				}
				time.Sleep(delayNano)

			}
		} else {
			if delayNano > time.Second {
				time.Sleep(delayNano)
			} else {
				time.Sleep(time.Second)
			}
		}
		totalErrors += errors
		log.Printf("Errors=%d (total=%d)\n", errors, totalErrors)
	}
	duration := time.Now().Sub(globalStart)
	for _, d := range links {
		if d.errs > 0 {
			stats(d, duration)
		}
		if d.Child().errs > 0 {
			stats(d.Child(), duration)
		}
	}
}

func bitStatus(val uint64, desc map[int]string) string {
	var res string
	for i := 0; val != 0; i++ {
		if val&1 != 0 {
			if s, ok := desc[i]; ok {
				res += s + "+ "
			} else {
				res += fmt.Sprintf("bit-%d ", i)
			}
		}
		val >>= 1
	}
	return res
}

func errBrowse(w *pci.World, clear bool) {
	for _, d := range w.Devs {
		var report []string
		stat := d.PciStatus()
		if stat&pci.PCI_STATUS_SERR != 0 {
			report = append(report, fmt.Sprintf("Sta: SERR"))
			if clear {
				d.Write16(pci.PCI_STATUS, pci.PCI_STATUS_SERR)
			}
		}
		if d.Ocap[pci.PCI_CAP_ID_EXP] > 0 {
			devStatReg := d.Ocap[pci.PCI_CAP_ID_EXP] + 0xa
			eStat := d.Read16(devStatReg) & 0xf &^ devExpErrMask
			if eStat != 0 {
				report = append(report, fmt.Sprintf("DevExtSta: %s", bitStatus(uint64(eStat), pciExpErrDesc)))
				if clear {
					d.Write16(devStatReg, eStat)
				}
			}
		}
		if d.Ecap[pci.PCI_ECAP_ID_AER] > 0 {
			aerUncReg := d.Ecap[pci.PCI_ECAP_ID_AER] + 0x4
			eStat := d.Read32(aerUncReg) &^ aerUncUrMask
			if eStat != 0 {
				report = append(report, fmt.Sprintf("AerUncSta: %s", bitStatus(uint64(eStat), aerUncErrDesc)))
				if clear {
					d.Write32(aerUncReg, eStat)
				}
			}
			aerCorrReg := d.Ecap[pci.PCI_ECAP_ID_AER] + 0x10
			eStat = d.Read32(aerCorrReg) &^ aerCorrUrMask
			if eStat != 0 {
				report = append(report, fmt.Sprintf("AerCorrSta: %s", bitStatus(uint64(eStat), aerCorrErrDesc)))
				if clear {
					d.Write32(aerCorrReg, eStat)
				}
			}
		}
		if report != nil {
			fmt.Printf("%s (%s):\n", d.Path, d.Name)
			for _, s := range report {
				fmt.Printf("    %s\n", s)
			}
		}
	}
}

//cgo export
func Main() {
	delayOpt := flag.Float64("delay", 0.01, "delay between polls of errored link for -mon")
	nbIter := flag.Int("iters", 5, "number of seconds/iterations for -mon")
	monLinkOpt := flag.Bool("mon", false, "monitor link for errors")
	errShowOpt := flag.Bool("err", false, "show err")
	clearErrOpt := flag.Bool("clearerr", false, "clear error status")
	optReportUR := flag.Bool("ur", false, "Do not ignore UR")
	flag.BoolVar(&verbose, "v", false, "more info")

	flag.Parse()

	if *optReportUR {
		aerCorrUrMask = 0
		aerUncUrMask = 0
		devExpErrMask = 0
	}

	world := pci.PciInit()

	if *monLinkOpt {
		monLinks(world, *nbIter, time.Duration(*delayOpt*1e9))
	}
	if *errShowOpt {
		errBrowse(world, true)
	}

	if *clearErrOpt {
		errBrowse(world, true)
	}
	if !*monLinkOpt && !*errShowOpt && !*clearErrOpt {
		showLinks(world, false)
	}
}

func main() {
	Main()
}
