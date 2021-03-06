package git

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"strconv"
	"strings"
)

const (
	// MaxPacketLength is the maximum total (header+payload) length
	// encode-able within one packet using Git's pkt-line protocol.
	MaxPacketLength = 65516
)

type pktline struct {
	r *bufio.Reader
	w *bufio.Writer
}

func newPktline(r io.Reader, w io.Writer) *pktline {
	return &pktline{
		r: bufio.NewReader(r),
		w: bufio.NewWriter(w),
	}
}

// readPacket reads a single packet entirely and returns the data encoded within
// it. Errors can occur in several cases, as described below.
//
// 1) If no data was present in the reader, and no more data could be read (the
//    pipe was closed, etc) than an io.EOF will be returned.
// 2) If there was some data to be read, but the pipe or reader was closed
//    before an entire packet (or header) could be ingested, an
//    io.ErrShortBuffer error will be returned.
// 3) If there was a valid header, but no body associated with the packet, an
//    "Invalid packet length." error will be returned.
// 4) If the data in the header could not be parsed as a hexadecimal length in
//    the Git pktline format, the parse error will be returned.
//
// If none of the above cases fit the state of the data on the wire, the packet
// is returned along with a nil error.
func (p *pktline) readPacket() ([]byte, error) {
	var pktLenHex [4]byte
	if n, err := io.ReadFull(p.r, pktLenHex[:]); err != nil {
		return nil, err
	} else if n != 4 {
		return nil, io.ErrShortBuffer
	}

	pktLen, err := strconv.ParseInt(string(pktLenHex[:]), 16, 0)
	if err != nil {
		return nil, err
	}

	// pktLen==0: flush packet
	if pktLen == 0 {
		return nil, nil
	}
	if pktLen <= 4 {
		return nil, errors.New("Invalid packet length.")
	}

	payload, err := ioutil.ReadAll(io.LimitReader(p.r, pktLen-4))
	return payload, err
}

func (p *pktline) readPacketText() (string, error) {
	data, err := p.readPacket()
	return strings.TrimSuffix(string(data), "\n"), err
}

func (p *pktline) readPacketList() ([]string, error) {
	var list []string
	for {
		data, err := p.readPacketText()
		if err != nil {
			return nil, err
		}

		if len(data) == 0 {
			break
		}

		list = append(list, data)
	}

	return list, nil
}

func (p *pktline) writePacket(data []byte) error {
	if len(data) > MaxPacketLength {
		return errors.New("Packet length exceeds maximal length")
	}

	if _, err := p.w.WriteString(fmt.Sprintf("%04x", len(data)+4)); err != nil {
		return err
	}

	if _, err := p.w.Write(data); err != nil {
		return err
	}

	if err := p.w.Flush(); err != nil {
		return err
	}

	return nil
}

func (p *pktline) writeFlush() error {
	if _, err := p.w.WriteString(fmt.Sprintf("%04x", 0)); err != nil {
		return err
	}

	if err := p.w.Flush(); err != nil {
		return err
	}

	return nil
}

func (p *pktline) writePacketText(data string) error {
	return p.writePacket([]byte(data + "\n"))
}

func (p *pktline) writePacketList(list []string) error {
	for _, i := range list {
		if err := p.writePacketText(i); err != nil {
			return err
		}
	}

	return p.writeFlush()
}
