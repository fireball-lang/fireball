package obj

import (
	"fmt"
	"io"
)

type CountedWriter struct {
	Out io.Writer
	n   uint64
}

func (c *CountedWriter) Write(p []byte) (int, error) {
	n, err := c.Out.Write(p)
	c.n += uint64(n)
	return n, err
}

var zeroChunk [4096]byte

func (c *CountedWriter) PadTo(target uint64) error {
	if target < c.n {
		return fmt.Errorf("obj.CountedWriter.PadTo() - cannot pad backwards: at %d, target %d", c.n, target)
	}

	for c.n < target {
		toWrite := min(target-c.n, uint64(len(zeroChunk)))

		if _, err := c.Write(zeroChunk[:toWrite]); err != nil {
			return err
		}
	}

	return nil
}
