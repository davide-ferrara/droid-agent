package term

import (
	"bufio"
	"fmt"
	"io"
	"log"
)

// KeyEvent describes one logical key. For KindPrintable the
// rune is in Rune (Byte is also set for the ASCII subset so
// existing ASCII-only callers keep working). For KindCtrl the
// raw control byte is in Byte. For KindCSI the parsed sequence
// tail is in Seq.
type KeyEvent struct {
	Kind byte
	Byte byte
	Rune rune
	Seq  string
}

const (
	KindPrintable byte = 'p'
	KindCtrl      byte = 'c'
	KindCSI       byte = 's'
	KindQuit      byte = 'q'
	KindEOF       byte = 'e'
)

func ReadKey(r *bufio.Reader) KeyEvent {
	b, err := r.ReadByte()
	if err == io.EOF {
		return KeyEvent{Kind: KindEOF}
	}
	if err != nil {
		log.Println(err)
		return KeyEvent{Kind: KindEOF}
	}

	// Multi-byte UTF-8 leading byte: put the byte back and let
	// bufio.Reader decode the full rune (handles 2-, 3-, and
	// 4-byte sequences, validates continuation bytes, and
	// replaces invalid encodings with U+FFFD so the caller
	// never sees a half-formed sequence).
	if b >= 0x80 {
		_ = r.UnreadByte()
		rn, _, err := r.ReadRune()
		if err != nil {
			log.Println(err)
			return KeyEvent{Kind: KindEOF}
		}
		return KeyEvent{Kind: KindPrintable, Rune: rn}
	}

	if b >= 0x20 && b < 0x7F {
		return KeyEvent{Kind: KindPrintable, Byte: b, Rune: rune(b)}
	}

	switch b {
	case Esc:
		seq, err := readCSISeq(r)
		if err != nil {
			err := fmt.Errorf("error reading stdin: %w", err)
			log.Println(err)
			return KeyEvent{Kind: KindCSI, Seq: string(seq)}
		}
		return KeyEvent{Kind: KindCSI, Seq: string(seq)}
	case CtrlC:
		return KeyEvent{Kind: KindQuit}
	default:
		return KeyEvent{Kind: KindCtrl, Byte: b}
	}
}

func readCSISeq(r *bufio.Reader) ([]byte, error) {
	seq := []byte{}
	for range 5 {
		b, err := r.ReadByte()
		if err != nil {
			return seq, err
		}
		seq = append(seq, b)
		switch {
		case b == '~':
			return seq, nil
		case b >= 0x41 && b <= 0x5A:
			if len(seq) == 1 && b == 'O' {
				continue
			}
			return seq, nil
		case b >= 0x61 && b <= 0x7A:
			return seq, nil
		}
	}
	return seq, nil
}
