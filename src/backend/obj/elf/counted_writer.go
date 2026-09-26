package elf

import (
	"fmt"
	"io"
)

type countedWriter struct {
	w io.Writer
	n uint64
}

func (c *countedWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += uint64(n)
	return n, err
}

var zeroChunk [4096]byte

func (c *countedWriter) PadTo(target uint64) error {
	if target < c.n {
		return fmt.Errorf("elf.countedWriter.PadTo() Cannot pad backwards: at %d, target %d", c.n, target)
	}

	for c.n < target {
		toWrite := min(target-c.n, uint64(len(zeroChunk)))

		if _, err := c.Write(zeroChunk[:toWrite]); err != nil {
			return err
		}
	}

	return nil
}
