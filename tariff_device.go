package iec62056

import (
	"errors"
	"time"
)

var ErrNoConnection = errors.New("connection is not set for tariff device")
var ErrInvalidPassword = errors.New("invalid password")

// PasswordFunc callback accepts operand for secure algorithm
// and returns encoded value.
// For clear text passwords return CommandId.CmdP1
// For encoded passwords using operand return CommandId.P2
type PasswordFunc func(arg DataSet) (DataSet, CommandId)

// TariffDevice is a client that communicates using IEC-62056-21 protocol.
type TariffDevice struct {
	// Device address
	Address string
	// Password callback
	Pass PasswordFunc
	//Timeout after device is reset from programming mode
	IdleTimeout time.Duration
	//TCP connection
	Connection *Conn
	// state flag
	programmingMode bool
	// last request timestamp
	lastActivity time.Time
	// Identity message received on handshake
	identity *Identity
}

func (t *TariffDevice) Identity() (Identity, error) {
	if t.identity != nil {
		return *t.identity, nil
	}
	if _, err := t.handShake(); err != nil {
		return Identity{}, err
	}
	return *t.identity, nil
}

func (t *TariffDevice) ReadOut(isModeD bool) (db DataBlock, err error) {
	if isModeD {
		return t.modeDreadOut()
	}
	data, err := t.handShake()
	if err != nil {
		return
	}

	if t.identity.Mode != ModeC {
		return *data, nil
	}
	data, err = t.Option(OptionSelectMessage{
		Option: DataReadOut,
		PCC:    NormalPCC,
	})
	if err != nil {
		return
	}
	return *data, nil
}

func (t *TariffDevice) Option(o OptionSelectMessage) (db *DataBlock, err error) {
	if t.identity == nil || t.isInProgrammingMode() {
		_, err = t.handShake()
		if err != nil {
			return
		}
	}
	if t.identity.Mode != ModeC {
		err = errors.New("Option selection is available for Mode C only")
		return
	}
	o.bri = t.identity.bri
	data, err := o.MarshalBinary()
	if err != nil {
		return
	}

	t.programmingMode = false
	data, err = t.cmd(data)
	if err != nil {
		return
	}

	if o.Option == ProgrammingMode {
		err = t.passExchange(data)
		return
	}
	err = db.UnmarshalBinary(data)
	return
}

func (t *TariffDevice) Command(cmd Command) (rs DataBlock, err error) {
	if !t.isInProgrammingMode() {
		if cmd.Id == CmdB0 {
			return
		}

		err = t.enterProgrammingMode()
		if err != nil {
			return
		}
	}

	if cmd.Id == CmdB0 {
		err = t.SendBreak()
		return
	}

	data, err := cmd.MarshalBinary()
	if err != nil {
		return
	}
	data, err = t.cmd(data)
	if err != nil {
		return
	}
	err = rs.UnmarshalBinary(data)
	return
}

func (t *TariffDevice) SendBreak() error {
	err := writeMessage(t.Connection, breakMsg)
	t.identity = nil
	t.programmingMode = false
	return err
}

func (t *TariffDevice) enterProgrammingMode() error {
	var noIdentity = t.identity == nil
	if noIdentity {
		_, err := t.handShake()
		if err != nil {
			return err
		}
	}
	if t.identity.Mode == ModeC {
		_, err := t.Option(OptionSelectMessage{
			Option: ProgrammingMode,
			PCC:    NormalPCC,
			bri:    t.identity.bri,
		})
		return err
	}

	if t.Pass == nil || t.identity.Mode != ModeB {
		return nil
	}
	ds := DataSet{
		Address: "",
		Value:   "",
		Unit:    "",
	}
	data, _ := ds.MarshalBinary()
	return t.passExchange(data)
}

func (t *TariffDevice) passExchange(p []byte) error {
	var ds DataSet
	err := ds.UnmarshalBinary(p)
	if err != nil {
		return err
	}

	if t.Pass == nil {
		t.programmingMode = true
		return nil
	}
	rv, cmd := t.Pass(ds)

	passCmd := &Command{
		Id:      cmd,
		Payload: &rv,
	}

	data, err := passCmd.MarshalBinary()
	if err != nil {
		return err
	}
	data, err = t.cmd(data)
	if err != nil {
		return err
	}

	if data[0] == ack {
		t.programmingMode = true
		return nil
	}
	var r DataSet
	if err = r.UnmarshalBinary(data); err != nil {
		return err
	}
	if r.Value != "" {
		return errors.New(r.Value)
	}
	return ErrInvalidPassword
}

func (t *TariffDevice) modeDreadOut() (db DataBlock, err error) {
	data, err := readMessage(t.Connection)
	if err != nil {
		return
	}
	var id Identity
	err = id.UnmarshalBinary(data)
	if err != nil {
		return
	}
	id.Mode = ModeD
	data, err = readMessage(t.Connection)
	if err != nil {
		return
	}

	if len(data) == 2 {
		data, err = readMessage(t.Connection)
		if err != nil {
			return
		}
	}
	err = db.UnmarshalBinary(data)
	if err != nil {
		return
	}
	t.identity = &id
	return
}

func (t *TariffDevice) isInProgrammingMode() bool {
	if t.identity == nil {
		return false
	}
	if t.IdleTimeout == 0 {
		t.IdleTimeout = defaultInactivityTo
	}
	if t.lastActivity.Add(t.IdleTimeout).Before(time.Now()) {
		return false
	}
	return t.programmingMode
}

func (t *TariffDevice) handShake() (db *DataBlock, err error) {
	t.identity = nil
	t.programmingMode = false

	data, _ := requestMessage(t.Address).MarshalBinary()
	data, err = t.cmd(data)
	if err != nil {
		return
	}
	var id Identity
	err = id.UnmarshalBinary(data)
	if err != nil {
		return
	}
	if id.Mode == ModeC {
		t.identity = &id
		return
	}
	// for ModeB there should be baudRate change, but we support tcp only.
	data, err = readMessage(t.Connection)
	if err != nil {
		return
	}
	t.lastActivity = time.Now()
	t.programmingMode = true
	err = db.UnmarshalBinary(data)
	if err != nil {
		return
	}
	t.identity = &id
	return
}

func (t *TariffDevice) cmd(p []byte) (data []byte, err error) {
	for i := 0; i < 5; i++ {
		err = writeMessage(t.Connection, p)
		if err != nil {
			return
		}
		data, err = readMessage(t.Connection)
		if err == nil {
			t.lastActivity = time.Now()
			return
		}

		if err == ErrNAK {
			continue
		}
		return
	}
	err = ErrNAK
	return
}

func readMessage(c *Conn) (data []byte, err error) {
	if c == nil {
		err = ErrNoConnection
		return
	}
	defer func() {
		if err == nil {
			c.logResponse()
		}
	}()
	if err = c.prepareRead(); err != nil {
		return
	}

	head, err := c.r.ReadByte()
	if err != nil {
		return
	}
	var delimiter byte
	switch head {
	case nak:
		err = ErrNAK
		fallthrough
	case ack:
		return []byte{head}, nil
	case stx, soh:
		delimiter = etx // only full blocks are supported
	default:
		delimiter = lf
	}
	data, err = c.r.ReadBytes(delimiter)
	if err != nil {
		return
	}

	if delimiter == etx {
		check, errRead := c.r.ReadByte()
		if errRead != nil {
			return nil, errRead
		}
		if check != bcc(data) {
			err = ErrBCC
		}
	}
	return
}

func writeMessage(c *Conn, data []byte) (err error) {
	if len(data) == 0 {
		return
	}

	if c == nil {
		err = ErrNoConnection
		return
	}

	if err = c.prepareWrite(); err != nil {
		return
	}

	if _, err = c.w.Write(data); err != nil {
		return
	}

	switch data[0] {
	case soh:
		err = c.w.WriteByte(bcc(data[1:]))
	case start, ack:
		_, err = c.w.Write(crlf)
	}
	if err != nil {
		return
	}
	err = c.w.Flush()
	if err == nil {
		c.logRequest()
	}
	return
}

// Calculates checksum
func bcc(data []byte) byte {
	var c byte
	for _, b := range data {
		c += b
	}
	return c & 0x7f
}
