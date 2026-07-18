package term

import (
	"bufio"
	"fmt"
	"io"
	"log"
)

type KeyEvent struct {
	Kind byte
	Byte byte
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

	if b >= 0x20 && b < 0x7F {
		return KeyEvent{Kind: KindPrintable, Byte: b}
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
