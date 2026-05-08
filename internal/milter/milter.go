package milter

import (
	"encoding/binary"
	"fmt"
)

const (
	cmdConnect   = 'C'
	cmdHelo      = 'H'
	cmdMail      = 'M'
	cmdRcpt      = 'R'
	cmdData      = 'T'
	cmdEOB       = 'E'
	cmdAbort     = 'A'
	cmdMacro     = 'D'
	cmdBody      = 'B'
	cmdEOH       = 'N'
	cmdHeader    = 'L'
	cmdQuit      = 'Q'
	cmdQuitNewCon = 'K'
	cmdSkip      = 'S'

	respContinue = 'c'
	respReject   = 'r'
	respTempfail = 't'
	respAccept   = 'a'
	respDiscard  = 'd'
)

type Action int

const (
	ActionContinue Action = iota
	ActionReject
	ActionTempfail
	ActionAccept
	ActionDiscard
	ActionUnknown
)

func ActionFromCode(code byte) Action {
	switch code {
	case respContinue:
		return ActionContinue
	case respReject:
		return ActionReject
	case respTempfail:
		return ActionTempfail
	case respAccept:
		return ActionAccept
	case respDiscard:
		return ActionDiscard
	default:
		return ActionUnknown
	}
}

type Packet struct {
	Command byte
	Data    []byte
}

func (p *Packet) Encode() ([]byte, error) {
	size := len(p.Data) + 1
	if size > 0x7fffffff {
		return nil, fmt.Errorf("packet too large")
	}

	data := make([]byte, 5+len(p.Data))
	binary.BigEndian.PutUint32(data[0:4], uint32(size))
	data[4] = p.Command
	copy(data[5:], p.Data)
	return data, nil
}

func DecodePacket(data []byte) (*Packet, error) {
	if len(data) < 5 {
		return nil, fmt.Errorf("packet too short")
	}

	size := binary.BigEndian.Uint32(data[0:4])
	if int(size) != len(data)-4 {
		return nil, fmt.Errorf("packet size mismatch")
	}

	return &Packet{
		Command: data[4],
		Data:    data[5:],
	}, nil
}

type Milter struct {
	handler Handler
}

type Handler interface {
	OnConnect(hostname string, family byte, port uint16, address string) Action
	OnHelo(helo string) Action
	OnMail(from string) Action
	OnRcpt(to string) Action
	OnData() Action
	OnEOB() Action
}

func NewMilter(handler Handler) *Milter {
	return &Milter{handler: handler}
}
