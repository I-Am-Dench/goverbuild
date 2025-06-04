package deferredwriter

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

type ArrayFunc = func(w *Writer, i int) error
type DeferredArrayFunc = func(w *Writer, i int) (hasData bool, err error)

type Writer struct {
	ws  io.WriteSeeker
	pos uint32

	order binary.ByteOrder

	Strings  map[string]uint32
	Deferred []struct {
		Home  uint32
		Value any
	}
}

func New(ws io.WriteSeeker, order binary.ByteOrder, pos uint32) *Writer {
	return &Writer{
		ws:    ws,
		pos:   pos,
		order: order,
	}
}

func (w *Writer) deferValue(home uint32, v any) error {
	if err := w.PutUint32(0); err != nil {
		return err
	}

	w.Deferred = append(w.Deferred, struct {
		Home  uint32
		Value any
	}{home, v})

	return nil
}

func (w *Writer) DeferString(s string) error {
	if err := w.deferValue(w.pos, s); err != nil {
		return fmt.Errorf("defer string: %v", err)
	}
	return nil
}

func (w *Writer) DeferInt64(i int64) error {
	if err := w.deferValue(w.pos, i); err != nil {
		return fmt.Errorf("defer int64: %v", err)
	}
	return nil
}

func (w *Writer) DeferUint64(i uint64) error {
	if err := w.deferValue(w.pos, i); err != nil {
		return fmt.Errorf("defer uint64: %v", err)
	}
	return nil
}

func (w *Writer) PutInt32(i int32) error {
	if err := binary.Write(w.ws, w.order, i); err != nil {
		return err
	}
	w.pos += 4

	return nil
}

func (w *Writer) PutUint32(i uint32) error {
	if err := binary.Write(w.ws, w.order, i); err != nil {
		return err
	}
	w.pos += 4

	return nil
}

func (w *Writer) PutFloat32(f float32) error {
	if err := binary.Write(w.ws, w.order, math.Float32bits(f)); err != nil {
		return err
	}
	w.pos += 4

	return nil
}

func (w *Writer) PutBool(b bool) error {
	var i uint32
	if b {
		i = 1
	}
	if err := binary.Write(w.ws, w.order, i); err != nil {
		return err
	}
	w.pos += 4

	return nil
}

func (w *Writer) writeString(pos uint32, s string) (n int, err error) {
	const alignment = 4

	if _, err := w.ws.Seek(int64(pos), io.SeekStart); err != nil {
		return 0, err
	}

	written, err := WriteZString(w.ws, s)
	if err != nil {
		return 0, err
	}
	n += written

	// Alignment padding; Suggested by documentation
	cur := pos + uint32(n)
	if padding := alignment - (cur % alignment); padding < alignment {
		zeros := [3]byte{}
		if _, err := w.ws.Write(zeros[:padding]); err != nil {
			return 0, err
		}
		n += int(padding)
	}

	return n, nil
}

func (w *Writer) flushString(home, pos uint32, s string) (n int, err error) {
	if w.Strings == nil {
		w.Strings = make(map[string]uint32)
	}

	address, ok := w.Strings[s]
	if !ok {
		written, err := w.writeString(pos, s)
		if err != nil {
			return 0, fmt.Errorf("flush string: %v", err)
		}
		n += written

		w.Strings[s] = pos
		address = pos
	}

	if _, err := w.ws.Seek(int64(home), io.SeekStart); err != nil {
		return 0, fmt.Errorf("flush string: %v", err)
	}

	if err := binary.Write(w.ws, w.order, address); err != nil {
		return 0, fmt.Errorf("flush string: %v", err)
	}

	return n, nil
}

func (w *Writer) flushInt64(home, pos uint32, v int64) (n int, err error) {
	if _, err := w.ws.Seek(int64(pos), io.SeekStart); err != nil {
		return 0, fmt.Errorf("flush int64: %v", err)
	}

	if err := binary.Write(w.ws, w.order, v); err != nil {
		return 0, fmt.Errorf("flush int64: %v", err)
	}
	n += 8

	if _, err := w.ws.Seek(int64(home), io.SeekStart); err != nil {
		return 0, fmt.Errorf("flush int64: %v", err)
	}

	if err := binary.Write(w.ws, w.order, pos); err != nil {
		return 0, fmt.Errorf("flush int64: %v", err)
	}

	return n, nil
}

func (w *Writer) flushUint64(home, pos uint32, v uint64) (n int, err error) {
	if _, err := w.ws.Seek(int64(pos), io.SeekStart); err != nil {
		return 0, fmt.Errorf("flush uint64: %v", err)
	}

	if err := binary.Write(w.ws, w.order, v); err != nil {
		return 0, fmt.Errorf("flush uint64: %v", err)
	}
	n += 8

	if _, err := w.ws.Seek(int64(home), io.SeekStart); err != nil {
		return 0, fmt.Errorf("flush uint64: %v", err)
	}

	if err := binary.Write(w.ws, w.order, pos); err != nil {
		return 0, fmt.Errorf("flush uint64: %v", err)
	}

	return n, nil
}

func (w *Writer) Flush() error {
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
			written, err = w.flushString(d.Home, w.pos, v)
		case int64:
			written, err = w.flushInt64(d.Home, w.pos, v)
		case uint64:
			written, err = w.flushUint64(d.Home, w.pos, v)
		default:
			panic(fmt.Errorf("attempted to flush unhandled type: %T", v))
		}

		if err != nil {
			return err
		}
		w.pos += uint32(written)
	}
	w.Deferred = w.Deferred[:0]

	return nil
}

func (w *Writer) Array(length int, f ArrayFunc) error {
	if err := w.PutUint32(w.pos + 4); err != nil {
		return fmt.Errorf("array: %v", err)
	}

	for i := 0; i < length; i++ {
		if err := f(w, i); err != nil {
			return fmt.Errorf("array: %v", err)
		}
	}

	return nil
}

func (w *Writer) DeferredArray(length int, f DeferredArrayFunc, withPointer bool, initial ...byte) error {
	if withPointer {
		if err := w.PutUint32(w.pos + 4); err != nil {
			return fmt.Errorf("deferred array: %v", err)
		}
	}

	arrayPos := int64(w.pos)

	b := make([]byte, length*4)
	if len(initial) > 0 {
		for i := range b {
			b[i] = initial[0]
		}
	}

	if _, err := w.ws.Write(b); err != nil {
		return fmt.Errorf("deferred array: %v", err)
	}
	w.pos += uint32(len(b))

	for i := 0; i < length; i++ {
		dataPos := w.pos

		hasData, err := f(w, i)
		if err != nil {
			return fmt.Errorf("deferred array: %d: %v", i, err)
		}

		if hasData {
			if _, err := w.ws.Seek(arrayPos, io.SeekStart); err != nil {
				return fmt.Errorf("deferred array: %d: %v", i, err)
			}

			if err := binary.Write(w.ws, w.order, dataPos); err != nil {
				return fmt.Errorf("deferred array: %d: %v", i, err)
			}

			if _, err := w.ws.Seek(int64(w.pos), io.SeekStart); err != nil {
				return fmt.Errorf("deferred array: %d: %v", i, err)
			}
		}
		arrayPos += 4
	}

	return nil
}

func WriteZString(w io.Writer, s string) (n int, err error) {
	written, err := w.Write([]byte(s))
	if err != nil {
		return 0, fmt.Errorf("write zstring: %v", err)
	}
	n += written

	if _, err := w.Write([]byte{0}); err != nil {
		return 0, fmt.Errorf("write zstring: %v", err)
	}
	n += 1

	return n, nil
}
