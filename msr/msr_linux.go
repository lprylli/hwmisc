package msr

import (
	"fmt"
	"os"
)

func msrFile(cpu int) string {
	return fmt.Sprintf("/dev/cpu/%d/msr", cpu)
}

func readAt(f *os.File, b []byte, n int64) (int, error) {
	return f.ReadAt(b, n)
}

func writeAt(f *os.File, b []byte, n int64) (int, error) {
	return f.WriteAt(b, n)
}
