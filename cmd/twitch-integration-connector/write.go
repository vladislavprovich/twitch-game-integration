package main

import (
	"encoding/binary"
	"io"
)

func writeAll(dst io.Writer, data []byte) error {
	nw := 0
	for nw < len(data) {
		w, err := dst.Write(data[nw:])
		if err != nil {
			return err
		}
		nw += w
	}
	return nil
}

func writeLenPrefixed(dst io.Writer, data []byte) error {
	buf := [2]byte{}
	binary.LittleEndian.PutUint16(buf[:], uint16(len(data)))
	if err := writeAll(dst, buf[:]); err != nil {
		return err
	}
	if err := writeAll(dst, data); err != nil {
		return err
	}
	return nil
}
