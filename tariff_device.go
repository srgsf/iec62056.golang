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
	connection *Conn
	// state flag
	programmingMode bool
	// last request timestamp
	lastActivity time.Time
	// Identity message received on handshake
	identity *Identity
}

func NewTariffDevice(conn *Conn, address string) *TariffDevice {
	return &TariffDevice{
		connection:  conn,
		Address:     address,
		IdleTimeout: defaultInactivityTo,
	}
}

func (t *TariffDevice) SetConnection(conn *Conn) {
	t.connection = conn
	t.lastActivity = time.Time{}
	t.programmingMode = false
}

func (t *TariffDevice) Disconnect() {
	if t.connection != nil {
		t.connection.Close()
		t.connection = nil
	}
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

func (t *TariffDevice) ReadOut(isModeD bool) (*DataBlock, error) {
	if isModeD {
		return t.modeDreadOut()
	}
	data, err := t.handShake()
	if err != nil {
		return nil, err
	}

	if t.identity.Mode != ModeC {
		return data, nil
	}
	data, err = t.Option(OptionSelectMessage{
		Option: DataReadOut,
		PCC:    NormalPCC,
	})
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (t *TariffDevice) Option(o OptionSelectMessage) (*DataBlock, error) {
	if t.identity == nil || t.isInProgrammingMode() {
		_, err := t.handShake()
		if err != nil {
			return nil, err
		}
	}
	if t.identity.Mode != ModeC {
		err := errors.New("Option selection is available for Mode C only")
		return nil, err
	}
	o.bri = t.identity.bri
	data, err := o.MarshalBinary()
	if err != nil {
		return nil, err
	}

	t.programmingMode = false
	data, err = t.cmd(data)
	if err != nil {
		return nil, err
	}

	if o.Option == ProgrammingMode {
		err = t.passExchange(data)
		return nil, err
	}
	var rv DataBlock
	err = rv.UnmarshalBinary(data)
	if err != nil {
		return nil, err
	}
	return &rv, nil
}

func (t *TariffDevice) Command(cmd Command) (*DataBlock, error) {
	if !t.isInProgrammingMode() {
		if cmd.Id == CmdB0 {
			return nil, nil
		}

		err := t.enterProgrammingMode()
		if err != nil {
			return nil, err
		}
	}

	if cmd.Id == CmdB0 {
		return nil, t.SendBreak()
	}

	data, err := cmd.MarshalBinary()
	if err != nil {
		return nil, err
	}
	data, err = t.cmd(data)
	if err != nil {
		return nil, err
	}
	var db DataBlock
	err = db.UnmarshalBinary(data)
	if err != nil {
		return nil, err
	}
	return &db, nil
}

func (t *TariffDevice) SendBreak() error {
	err := writeMessage(t.connection, breakMsg)
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

func (t *TariffDevice) modeDreadOut() (*DataBlock, error) {
	data, err := readMessage(t.connection)
	if err != nil {
		return nil, err
	}
	var id Identity
	err = id.UnmarshalBinary(data)
	if err != nil {
		return nil, err
	}
	id.Mode = ModeD
	data, err = readMessage(t.connection)
	if err != nil {
		return nil, err
	}

	if len(data) == 2 {
		data, err = readMessage(t.connection)
		if err != nil {
			return nil, err
		}
	}
	var b DataBlock

	err = b.UnmarshalBinary(data)
	if err != nil {
		return nil, err
	}
	t.identity = &id
	return &b, nil
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

func (t *TariffDevice) handShake() (*DataBlock, error) {
	t.identity = nil
	t.programmingMode = false

	data, _ := requestMessage(t.Address).MarshalBinary()
	data, err := t.cmd(data)
	if err != nil {
		return nil, err
	}
	var id Identity
	err = id.UnmarshalBinary(data)
	if err != nil {
		return nil, err
	}
	if id.Mode == ModeC {
		t.identity = &id
		return nil, nil
	}
	// for ModeB there should be baudRate change, but we support tcp only.
	data, err = readMessage(t.connection)
	if err != nil {
		return nil, err
	}
	t.lastActivity = time.Now()
	t.programmingMode = true
	var b DataBlock
	err = b.UnmarshalBinary(data)
	if err != nil {
		return nil, err
	}
	t.identity = &id
	return &b, err
}

func (t *TariffDevice) cmd(p []byte) ([]byte, error) {
	for i := 0; i < 5; i++ {
		err := writeMessage(t.connection, p)
		if err != nil {
			return nil, err
		}
		data, err := readMessage(t.connection)
		if err == nil {
			t.lastActivity = time.Now()
			return data, nil
		}

		if err == ErrNAK {
			continue
		}
		return nil, err
	}
	return nil, ErrNAK
}

func readMessage(c *Conn) ([]byte, error) {
	if c == nil {
		err := ErrNoConnection
		return nil, err
	}

	if err := c.prepareRead(); err != nil {
		return nil, err
	}

	data, err := func(c *Conn) ([]byte, error) {
		head, err := c.r.ReadByte()
		if err != nil {
			return nil, err
		}
		var delimiter byte
		switch head {
		case nak:
			err = ErrNAK
			fallthrough
		case ack:
			return []byte{head}, err
		case stx, soh:
			delimiter = etx // only full blocks are supported
		default:
			delimiter = lf
		}
		data, err := c.r.ReadBytes(delimiter)
		if err != nil {
			return nil, err
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
		return data, err
	}(c)

	if err == nil {
		c.logResponse()
	}
	return data, err
}

func writeMessage(c *Conn, data []byte) error {
	if len(data) == 0 {
		return nil
	}

	if c == nil {
		return ErrNoConnection
	}

	if err := c.prepareWrite(); err != nil {
		return err
	}

	if _, err := c.w.Write(data); err != nil {
		return err
	}
	var err error

	switch data[0] {
	case soh:
		err = c.w.WriteByte(bcc(data[1:]))
	case start, ack:
		_, err = c.w.Write(crlf)
	}
	if err != nil {
		return err
	}
	err = c.w.Flush()
	if err == nil {
		c.logRequest()
	}
	return err
}

// Calculates checksum
func bcc(data []byte) byte {
	var c byte
	for _, b := range data {
		c += b
	}
	return c & 0x7f
}
