package fdb

import (
	"encoding/binary"
	"fmt"
	"io"
)

type deferredWriter struct {
	io.WriteSeeker

	pos uint32
}

func newDeferredWriter(w io.WriteSeeker, pos uint32) deferredWriter {
	return deferredWriter{w, pos}
}

func (w *deferredWriter) Pos() uint32 {
	return w.pos
}

func (w *deferredWriter) Array(length, stride int, f func(w io.WriteSeeker, i int) (n int64, err error), initial ...byte) error {
	arrayPos := int64(w.pos)

	b := make([]byte, length*stride)

	if len(initial) > 0 {
		for i := range b {
			b[i] = initial[0]
		}
	}

	if _, err := w.Write(b); err != nil {
		return fmt.Errorf("array: %w", err)
	}
	w.pos += uint32(len(b))

	for i := 0; i < length; i++ {
		if _, err := w.Seek(arrayPos, io.SeekStart); err != nil {
			return fmt.Errorf("array: %w", err)
		}

		if err := binary.Write(w, order, w.pos); err != nil {
			return fmt.Errorf("array: %w", err)
		}

		if _, err := w.Seek(int64(w.pos), io.SeekStart); err != nil {
			return fmt.Errorf("array: %w", err)
		}

		written, err := f(w, i)
		if err != nil {
			return fmt.Errorf("array: %w", err)
		}
		w.pos += uint32(written)

		arrayPos += int64(stride)
	}

	return nil
}

func (w *deferredWriter) PutUint32(v uint32) error {
	if err := binary.Write(w, order, v); err != nil {
		return err
	}
	w.pos += 4

	return nil
}

type stringPool struct {
	Strings  map[string]uint32
	Deferred map[uint32]string
}

func (p *stringPool) Add(home uint32, s string) {
	if p.Deferred == nil {
		p.Deferred = make(map[uint32]string)
	}

	p.Deferred[home] = s
}

func (p *stringPool) writeString(w io.WriteSeeker, s string, address uint32) (written int, err error) {
	const alignment = 4

	if _, err := w.Seek(int64(address), io.SeekStart); err != nil {
		return 0, err
	}

	n, err := WriteNullTerminatedString(s, w)
	if err != nil {
		return 0, err
	}
	written += n

	cur := address + uint32(written)
	if padding := alignment - (cur % alignment); padding < alignment {
		zeros := [3]byte{}
		if _, err := w.Write(zeros[:padding]); err != nil {
			return 0, err
		}
		written += int(padding)
	}

	return written, nil
}

func (p *stringPool) Flush(w io.WriteSeeker, base uint32) (n int, err error) {
	if p.Deferred == nil {
		return 0, nil
	}

	for home, text := range p.Deferred {
		if p.Strings == nil {
			p.Strings = make(map[string]uint32)
		}

		address, ok := p.Strings[text]
		if !ok {
			written, err := p.writeString(w, text, base)
			if err != nil {
				return 0, fmt.Errorf("string pool: flush: %v", err)
			}
			n += written

			p.Strings[text] = base
			address = base

			base += uint32(written)
		}

		if _, err := w.Seek(int64(home), io.SeekStart); err != nil {
			return 0, fmt.Errorf("string pool: flush: %v", err)
		}

		if err := binary.Write(w, order, address); err != nil {
			return 0, fmt.Errorf("string pool: flush: %v", err)
		}
	}

	clear(p.Deferred)
	return n, nil
}

type writer struct {
	io.WriteSeeker
	pos uint32

	Strings  map[string]uint32
	Deferred []struct {
		Address uint32
		Value   any
	}
}

func (w *writer) deferValue(address uint32, v any) {
	w.Deferred = append(w.Deferred, struct {
		Address uint32
		Value   any
	}{address, v})
}

func (w *writer) AddString(s string) {
	w.deferValue(w.pos, s)
}

func (w *writer) AddInt64(i int64) {
	w.deferValue(w.pos, i)
}

func (w *writer) PutUint32(i uint32) error {
	if err := binary.Write(w, order, i); err != nil {
		return err
	}
	w.pos += 4

	return nil
}

func (w *writer) writeString(s string, pos uint32) (n int, err error) {
	const alignment = 4

	if _, err := w.Seek(int64(pos), io.SeekStart); err != nil {
		return 0, err
	}

	written, err := WriteNullTerminatedString(s, w)
	if err != nil {
		return 0, err
	}
	n += written

	// Alignment padding; Suggested by documentation
	cur := pos + uint32(n)
	if padding := alignment - (cur % alignment); padding < alignment {
		zeros := [3]byte{}
		if _, err := w.Write(zeros[:padding]); err != nil {
			return 0, err
		}
		n += int(padding)
	}

	return n, nil
}

func (w *writer) flushString(s string, home, pos uint32) (n int, err error) {
	if w.Strings == nil {
		w.Strings = make(map[string]uint32)
	}

	address, ok := w.Strings[s]
	if !ok {
		written, err := w.writeString(s, pos)
		if err != nil {
			return 0, fmt.Errorf("flush string: %v", err)
		}
		n += written

		w.Strings[s] = pos
		address = pos
	}

	if _, err := w.Seek(int64(home), io.SeekStart); err != nil {
		return 0, fmt.Errorf("flush string: %v", err)
	}

	if err := binary.Write(w, order, address); err != nil {
		return 0, fmt.Errorf("flush string: %v", err)
	}

	return n, nil
}

func (w *writer) flushInt64(i int64) (n int, err error) {
	if err := binary.Write(w, order, i); err != nil {
		return 0, fmt.Errorf("flush int64: %v", err)
	}

	return 8, nil
}

func (w *writer) Flush() error {
	if w.Deferred == nil {
		return nil
	}

	for _, d := range w.Deferred {
		var (
			written int
			err     error
		)

		switch v := d.Value.(type) {
		case string:
			written, err = w.flushString(v, d.Address, w.pos)
		case int64:
			written, err = w.flushInt64(v)
		default:
			panic(fmt.Errorf("attempted to flush unhandled type: %T", v))
		}

		if err != nil {
			return err
		}
		w.pos += uint32(written)
	}

	return nil
}

// func (w *writer) Array(length, stride int, f func(w *writer, i int) error, initial ...byte) error {
// 	arrayPos := int64(w.pos)

// 	b := make([]byte, length*stride)
// }
