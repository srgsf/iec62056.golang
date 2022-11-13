package iec62056

import (
	"bufio"
	"bytes"
	"errors"
	"time"
)

const defaultInactivityTo = 120 * time.Second

const (
	start = 0x2f
	end   = 0x21
	ack   = 0x06
	soh   = 0x01
	stx   = 0x02
	etx   = 0x03
	trc   = 0x3f
	nak   = 0x15
	cr    = 0x0d
	lf    = 0x0a
	fb    = 0x28
	rb    = 0x29
	star  = 0x2a
)

type ProtocolMode byte

const (
	ModeA ProtocolMode = 'A' + iota
	ModeB
	ModeC
	ModeD
)

type PCC byte

const (
	NormalPCC    PCC = '0'
	SecondaryPCC PCC = '1'
)

type Option byte

const (
	DataReadOut Option = '0' + iota
	ProgrammingMode
	_ //reserved
	_ //reserved
	_ //reserved
	_ //reserved
	Option6
	Option7
	Option8
	Option9
)

type CommandId int

const (
	CmdP0 CommandId = iota
	CmdP1
	CmdP2
	CmdW1
	CmdW2
	CmdR1
	CmdR2
	CmdE2
	CmdB0
)

var commands = map[CommandId][2]byte{
	CmdP0: {'P', '0'},
	CmdP1: {'P', '1'},
	CmdP2: {'P', '2'},
	CmdW1: {'W', '1'},
	CmdW2: {'W', '2'},
	CmdR1: {'R', '1'},
	CmdR2: {'R', '2'},
	CmdE2: {'E', '2'},
	CmdB0: {'B', '0'},
}

var crlf = []byte{cr, lf}
var breakMsg = []byte{soh, commands[CmdB0][0], commands[CmdB0][1], etx}

var ErrBCC = errors.New("checksum failed")
var ErrNAK = errors.New("nak received")

type DataBlock struct {
	Lines []DataLine
}

type DataLine struct {
	Sets []DataSet
}

type DataSet struct {
	Address string
	Value   string
	Unit    string
}

type Command struct {
	Id      CommandId
	Payload *DataSet
}

type OptionSelectMessage struct {
	Option        Option
	PCC           PCC
	bri           byte
	skipHandShake bool
}

type requestMessage string

type Identity struct {
	Device       string
	Manufacturer string
	Mode         ProtocolMode
	bri          byte
}

func (ds *DataSet) MarshalBinary() ([]byte, error) {
	length := len(ds.Address)
	length += len(ds.Value)
	unitLen := len(ds.Unit)
	if length+unitLen == 0 {
		return []byte{}, nil
	}
	length += unitLen + 3 //fb+rb+star
	rv := make([]byte, 0, length)
	rv = append(rv, ds.Address...)
	rv = append(rv, fb)
	rv = append(rv, ds.Value...)
	if unitLen != 0 {
		rv = append(rv, star)
		rv = append(rv, ds.Unit...)
	}
	rv = append(rv, rb)
	return rv, nil
}

func (ds *DataSet) UnmarshalBinary(data []byte) error {
	fbIdx := bytes.IndexByte(data, fb)
	if fbIdx == -1 {
		return errors.New("front boundary is missing")
	}
	rbIdx := bytes.IndexByte(data, rb)
	if rbIdx == -1 {
		return errors.New("rear boundary is missing")
	}
	*ds = DataSet{}
	ds.Address = ""
	if fbIdx > 0 {
		ds.Address = string(data[0:fbIdx])
	}

	data = data[fbIdx+1 : rbIdx]

	spIdx := bytes.IndexByte(data, star)
	ds.Value = ""
	ds.Unit = ""
	if spIdx == -1 {
		ds.Value = string(data)
		return nil
	}

	ds.Value = string(data[:spIdx])
	ds.Unit = string(data[spIdx+1:])
	return nil
}

func (dl *DataLine) UnmarshalBinary(data []byte) error {
	s := bufio.NewScanner(bytes.NewReader(data))
	s.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}
		if i := bytes.IndexByte(data, rb); i >= 0 {
			return i + 1, data[:i+1], nil
		}

		if atEOF {
			return len(data), data, errors.New("invalid data set")
		}
		return 0, nil, nil
	})
	*dl = DataLine{}
	for s.Scan() {
		if s.Err() != nil {
			return s.Err()
		}
		var ds DataSet
		if err := ds.UnmarshalBinary(s.Bytes()); err != nil {
			return err
		}
		dl.Sets = append(dl.Sets, ds)
	}
	return nil
}

func (db *DataBlock) UnmarshalBinary(data []byte) error {
	s := bufio.NewScanner(bytes.NewReader(data))
	*db = DataBlock{}
	for s.Scan() {
		if s.Err() != nil {
			return s.Err()
		}
		var dl DataLine
		if err := dl.UnmarshalBinary(s.Bytes()); err != nil {
			return err
		}
		db.Lines = append(db.Lines, dl)
	}
	return nil
}

func (c *Command) MarshalBinary() ([]byte, error) {
	var plLen int
	var pl []byte
	if c.Payload != nil {
		pl, _ = c.Payload.MarshalBinary()
		plLen = len(pl)
	}
	cmd, ok := commands[c.Id]
	if !ok {
		return nil, errors.New("invalid command")
	}
	rv := make([]byte, 0, plLen+6)
	rv = append(rv, soh, cmd[0], cmd[1])
	if plLen != 0 {
		rv = append(rv, stx)
		rv = append(rv, pl...)
	}
	return append(rv, etx), nil
}

func (o *OptionSelectMessage) MarshalBinary() ([]byte, error) {
	return []byte{ack, byte(o.PCC), o.bri, byte(o.Option)}, nil
}

func (address requestMessage) MarshalBinary() ([]byte, error) {
	l := len(address)
	if l == 0 {
		return []byte{start, trc, end}, nil
	}
	msg := make([]byte, 0, l+5) //cr lf will be added
	msg = append(msg, start, trc)
	msg = append(msg, address...)
	return append(msg, end), nil
}

func (id *Identity) UnmarshalBinary(data []byte) error {
	if len(data) < 4 {
		return errors.New("identity message too short")
	}
	*id = Identity{}

	id.Manufacturer = string(data[0:3])
	id.bri = data[3]
	id.Mode = decodeMode(data[3])
	id.Device = string(data[4 : len(data)-2])
	return nil
}

func decodeMode(b byte) ProtocolMode {
	switch {
	case '0' <= b && b <= '9':
		return ModeC
	case 'A' <= b && b <= 'I':
		return ModeB
	}
	return ModeA
}

func decodeBaudRate(b byte) int {
	switch b {
	case 'A', '1':
		return 600
	case 'B', '2':
		return 1200
	case 'C', '3':
		return 2400
	case 'D', '4':
		return 4800
	case 'E', '5':
		return 9600
	}
	return 300
}
