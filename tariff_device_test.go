package iec62056

import (
	"bytes"
	"net"
	"reflect"
	"testing"
	"time"
)

var listener net.Listener
var ch chan net.Conn

func TestMain(m *testing.M) {
	listener, _ = net.Listen("tcp", "127.0.0.1:0")
	ch = make(chan net.Conn, 1)
	m.Run()
	listener.Close()
	close(ch)
}

func listen() (net.Conn, Conn) {
	go func() {
		rv, _ := listener.Accept()
		ch <- rv
	}()
	conn, _ := DialTCP(listener.Addr().String())
	return <-ch, conn
}

func getClosedConn() Conn {
	server, client := listen()
	server.Close()
	client.Close()
	return client
}

func TestNewTariffDevice(t *testing.T) {
	conn := getClosedConn()
	type args struct {
		conn Conn
	}
	tests := []struct {
		name string
		args args
		want *TariffDevice
	}{
		{
			name: "Just connection",
			args: args{conn},
			want: &TariffDevice{
				IdleTimeout:     120 * time.Second,
				connection:      conn,
				programmingMode: false,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewTariffDevice(tt.args.conn); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewTariffDevice() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWithAddress(t *testing.T) {
	conn := getClosedConn()
	type args struct {
		conn    Conn
		address string
	}
	tests := []struct {
		name string
		args args
		want *TariffDevice
	}{
		{
			name: "Addr",
			args: args{conn, "12345678"},
			want: &TariffDevice{
				IdleTimeout:     defaultInactivityTo,
				address:         "12345678",
				connection:      conn,
				programmingMode: false,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := WithAddress(tt.args.conn, tt.args.address); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("WithAddress() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWithPassword(t *testing.T) {
	conn := getClosedConn()
	callback := func(arg DataSet) (DataSet, CommandId) {
		return DataSet{Value: "vl"}, CmdP1
	}
	address := "address"
	type args struct {
		conn         Conn
		address      string
		passCallback PasswordFunc
	}
	tests := []struct {
		name string
		args args
		want *TariffDevice
	}{
		{
			name: "All",
			args: args{
				conn:         conn,
				address:      address,
				passCallback: callback,
			},
			want: &TariffDevice{
				IdleTimeout: defaultInactivityTo,
				address:     address,
				pass:        callback,
				connection:  conn,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WithPassword(tt.args.conn, tt.args.address, tt.args.passCallback)
			ds, cmd := got.pass(DataSet{})
			wds, wcmd := tt.args.passCallback(DataSet{})
			if !reflect.DeepEqual(ds, wds) || cmd != wcmd {
				t.Errorf("WithPassword() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTariffDevice_Reset(t *testing.T) {
	conn := getClosedConn()
	td := NewTariffDevice(conn)
	td.programmingMode = true
	conn2 := getClosedConn()
	td.Reset(conn2)
	if td.programmingMode {
		t.Error("programming mode is not cleared on reset")
	}
	if td.connection != conn2 {
		t.Error("connection is not replaced on reset")
	}
}

func TestTariffDevice_DropProgrammingMode(t *testing.T) {
	conn := getClosedConn()
	td := NewTariffDevice(conn)
	td.programmingMode = true
	td.DropProgrammingMode()
	if td.programmingMode {
		t.Error("programming mode is not cleared")
	}
}

func TestTariffDevice_Identity(t *testing.T) {
	server, conn := listen()
	defer server.Close()
	defer conn.Close()

	type fields struct {
		identity *Identity
	}
	tests := []struct {
		name    string
		fields  fields
		want    Identity
		setup   func()
		wantErr bool
	}{
		{
			name:    "Already read",
			fields:  fields{&Identity{Manufacturer: "MFK"}},
			want:    Identity{Manufacturer: "MFK"},
			wantErr: false,
		},
		{
			name:   "Received",
			fields: fields{},
			want: Identity{
				Device:       "test",
				Manufacturer: "iek",
				Mode:         ModeC,
				bri:          '4',
			},
			setup: func() {
				buf := make([]byte, 20)
				_, _ = server.Read(buf)
				_, _ = server.Write([]byte("/iek4test\r\n"))
			},
			wantErr: false,
		},
		{
			name:   "Error from conn",
			fields: fields{},
			want:   Identity{},
			setup: func() {
				server.Close()
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := NewTariffDevice(conn)
			if tt.setup != nil {
				go tt.setup()
			}
			td.identity = tt.fields.identity
			got, err := td.Identity()
			if (err != nil) != tt.wantErr {
				t.Errorf("TariffDevice.Identity() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TariffDevice.Identity() = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestTariffDevice_ReadOut(t *testing.T) {
	server, client := listen()
	defer client.Close()
	defer server.Close()
	tests := []struct {
		name    string
		fn      func(t *testing.T)
		want    *DataBlock
		wantErr bool
	}{
		{
			name: "ModeA",
			fn: func(_ *testing.T) {
				buf := make([]byte, 5)
				_, _ = server.Read(buf)
				_, _ = server.Write([]byte("/iekXtest\r\n"))
				var b bytes.Buffer
				b.WriteString("Data()!\r\n")
				b.WriteByte(etx)
				_, _ = server.Write([]byte{stx})
				_, _ = server.Write(b.Bytes())
				_, _ = server.Write([]byte{bcc(b.Bytes())})
			},
			want: &DataBlock{Lines: []DataLine{
				{
					Sets: []DataSet{
						{Address: "Data"},
					},
				},
			},
			},
			wantErr: false,
		},
		{
			name: "ModeB",
			fn: func(_ *testing.T) {
				buf := make([]byte, 5)
				_, _ = server.Read(buf)
				_, _ = server.Write([]byte("/iekEtest\r\n"))
				var b bytes.Buffer
				b.WriteString("Data(Val)!\r\n")
				b.WriteByte(etx)
				_, _ = server.Write([]byte{stx})
				_, _ = server.Write(b.Bytes())
				_, _ = server.Write([]byte{bcc(b.Bytes())})
			},
			want: &DataBlock{Lines: []DataLine{
				{
					Sets: []DataSet{
						{Address: "Data", Value: "Val"},
					},
				},
			},
			},
			wantErr: false,
		},
		{
			name: "ModeC",
			fn: func(t *testing.T) {
				buf := make([]byte, 5)
				_, _ = server.Read(buf)
				_, _ = server.Write([]byte("/iek6test\r\n"))
				buf = make([]byte, 6)
				_, _ = server.Read(buf)
				if !reflect.DeepEqual(buf, []byte{ack, '0', '6', '0', cr, lf}) {
					t.Fatalf("Invalid option message.")
				}
				var b bytes.Buffer
				b.WriteString("Data(Val)!\r\n")
				b.WriteByte(etx)
				_, _ = server.Write([]byte{stx})
				_, _ = server.Write(b.Bytes())
				_, _ = server.Write([]byte{bcc(b.Bytes())})
			},
			want: &DataBlock{Lines: []DataLine{
				{
					Sets: []DataSet{
						{Address: "Data", Value: "Val"},
					},
				},
			},
			},
			wantErr: false,
		},
		{
			name: "ModeC Error",
			fn: func(t *testing.T) {
				buf := make([]byte, 5)
				_, _ = server.Read(buf)
				_, _ = server.Write([]byte("/iek6test\r\n"))
				buf = make([]byte, 6)
				_, _ = server.Read(buf)
				if !reflect.DeepEqual(buf, []byte{ack, '0', '6', '0', cr, lf}) {
					t.Fatalf("Invalid option message.")
				}
				var b bytes.Buffer
				b.WriteString("Data(Val!\r\n")
				b.WriteByte(etx)
				_, _ = server.Write([]byte{stx})
				_, _ = server.Write(b.Bytes())
				_, _ = server.Write([]byte{bcc(b.Bytes())})
			},
			want: &DataBlock{Lines: []DataLine{
				{
					Sets: []DataSet{
						{Address: "Data", Value: "Val"},
					},
				},
			},
			},
			wantErr: true,
		},
		{
			name: "handShake fail",
			fn: func(_ *testing.T) {
				buf := make([]byte, 5)
				_, _ = server.Read(buf)
				_, _ = server.Write([]byte("\r\n"))
			},
			want: &DataBlock{Lines: []DataLine{
				{
					Sets: []DataSet{
						{Address: "Data", Value: "Val"},
					},
				},
			},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := NewTariffDevice(client)
			go tt.fn(t)
			got, err := tr.ReadOut()
			if (err != nil) != tt.wantErr {
				t.Errorf("TariffDevice.ReadOut() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TariffDevice.ReadOut() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTariffDevice_Option(t *testing.T) {
	server, client := listen()
	defer client.Close()
	defer server.Close()
	type args struct {
		o OptionSelectMessage
	}
	tests := []struct {
		name    string
		args    args
		fn      func(t *testing.T)
		want    *DataBlock
		wantErr bool
	}{
		{
			name: "Mode error",
			args: args{
				OptionSelectMessage{
					Option: Option6,
					PCC:    NormalPCC,
				},
			},
			fn: func(_ *testing.T) {
				buf := make([]byte, 5)
				_, _ = server.Read(buf)
				_, _ = server.Write([]byte("/iekAtest\r\n"))
				var b bytes.Buffer
				b.WriteString("Data(Val)!\r\n")
				b.WriteByte(etx)
				_, _ = server.Write([]byte{stx})
				_, _ = server.Write(b.Bytes())
				_, _ = server.Write([]byte{bcc(b.Bytes())})
			},
			want:    &DataBlock{},
			wantErr: true,
		},
		{
			name: "Option 6",
			args: args{
				OptionSelectMessage{
					Option: Option6,
					PCC:    NormalPCC,
				},
			},
			fn: func(_ *testing.T) {
				buf := make([]byte, 5)
				_, _ = server.Read(buf)
				_, _ = server.Write([]byte("/iek6test\r\n"))
				buf = make([]byte, 6)
				_, _ = server.Read(buf)
				if !reflect.DeepEqual(buf, []byte{ack, '0', '6', '6', cr, lf}) {
					t.Fatalf("Invalid option message.")
				}
				var b bytes.Buffer
				b.WriteString("Data(Val)!\r\n")
				b.WriteByte(etx)
				_, _ = server.Write([]byte{stx})
				_, _ = server.Write(b.Bytes())
				_, _ = server.Write([]byte{bcc(b.Bytes())})
			},
			want: &DataBlock{Lines: []DataLine{
				{
					Sets: []DataSet{
						{Address: "Data", Value: "Val"},
					},
				},
			},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := NewTariffDevice(client)
			go tt.fn(t)
			got, err := tr.Option(tt.args.o)
			if (err != nil) != tt.wantErr {
				t.Errorf("TariffDevice.Option() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TariffDevice.Option() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTariffDevice_Command(t *testing.T) {
	server, client := listen()
	defer client.Close()
	defer server.Close()
	type fields struct {
		programmingMode bool
		lastActivity    time.Time
		identity        *Identity
	}
	tests := []struct {
		name    string
		fn      func(t *testing.T)
		cmd     Command
		fields  fields
		want    *DataBlock
		wantErr bool
	}{
		{
			name: "In programming mode",
			fields: fields{
				programmingMode: true,
				lastActivity:    time.Now(),
				identity:        &Identity{bri: '6'},
			},
			fn: func(_ *testing.T) {
				var b bytes.Buffer
				b.WriteString("Data(Val)!\r\n")
				b.WriteByte(etx)
				_, _ = server.Write([]byte{stx})
				_, _ = server.Write(b.Bytes())
				_, _ = server.Write([]byte{bcc(b.Bytes())})
				buf := make([]byte, 15)
				_, _ = server.Read(buf)
			},
			cmd: Command{
				Id: CmdR1,
				Payload: &DataSet{
					Address: "ADDR",
				},
			},
			want: &DataBlock{Lines: []DataLine{
				{
					Sets: []DataSet{
						{Address: "Data", Value: "Val"},
					},
				},
			},
			},
			wantErr: false,
		},
		{
			name: "Err in programming mode",
			fields: fields{
				programmingMode: true,
				lastActivity:    time.Now(),
				identity:        &Identity{bri: '6'},
			},
			fn: func(_ *testing.T) {
				var b bytes.Buffer
				b.WriteString("Data(Val\r\n")
				b.WriteByte(etx)
				_, _ = server.Write([]byte{stx})
				_, _ = server.Write(b.Bytes())
				_, _ = server.Write([]byte{bcc(b.Bytes())})
				buf := make([]byte, 15)
				_, _ = server.Read(buf)
			},
			cmd: Command{
				Id: CmdR1,
				Payload: &DataSet{
					Address: "ADDR",
				},
			},
			want: &DataBlock{Lines: []DataLine{
				{
					Sets: []DataSet{
						{Address: "Data", Value: "Val"},
					},
				},
			},
			},
			wantErr: true,
		},
		{
			name: "Not in programming mode",
			fields: fields{
				programmingMode: false,
				lastActivity:    time.Now(),
				identity:        &Identity{bri: '6'},
			},
			fn: func(_ *testing.T) {
				buf := make([]byte, 15)
				_, _ = server.Read(buf)
				_, _ = server.Write([]byte("/iek6test\r\n"))
				_, _ = server.Read(buf)
				p0 := []byte{'P', '0', stx, fb, '1', '2', '3', rb, etx}
				_, _ = server.Write([]byte{soh})
				_, _ = server.Write(p0)
				_, _ = server.Write([]byte{bcc(p0)})
				_, _ = server.Read(buf)
				var b bytes.Buffer
				b.WriteString("Data(Val)\r\n")
				b.WriteByte(etx)
				_, _ = server.Write([]byte{stx})
				_, _ = server.Write(b.Bytes())
				_, _ = server.Write([]byte{bcc(b.Bytes())})
			},
			cmd: Command{
				Id: CmdR1,
				Payload: &DataSet{
					Address: "ADDR",
				},
			},
			want: &DataBlock{Lines: []DataLine{
				{
					Sets: []DataSet{
						{Address: "Data", Value: "Val"},
					},
				},
			},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := NewTariffDevice(client)
			tr.identity = tt.fields.identity
			tr.programmingMode = tt.fields.programmingMode
			tr.lastActivity = tt.fields.lastActivity
			go tt.fn(t)
			got, err := tr.Command(tt.cmd)
			if (err != nil) != tt.wantErr {
				t.Errorf("TariffDevice.Command() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TariffDevice.Command() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTariffDevice_enterProgrammingMode(t *testing.T) {
	server, client := listen()
	defer client.Close()
	defer server.Close()
	type fields struct {
		pass         PasswordFunc
		lastActivity time.Time
	}
	tests := []struct {
		name    string
		fields  fields
		fn      func()
		wantErr bool
	}{
		{
			name: "No pass",
			fields: fields{
				pass:         nil,
				lastActivity: time.Now(),
			},
			fn: func() {
				buf := make([]byte, 15)
				_, _ = server.Read(buf)
				_, _ = server.Write([]byte("/iek6test\r\n"))
				_, _ = server.Read(buf)
				p0 := []byte{'P', '0', stx, fb, '1', '2', '3', rb, etx}
				_, _ = server.Write([]byte{soh})
				_, _ = server.Write(p0)
				_, _ = server.Write([]byte{bcc(p0)})
			},
			wantErr: false,
		},
		{
			name: "No pass B mode",
			fields: fields{
				pass:         nil,
				lastActivity: time.Now(),
			},
			fn: func() {
				_, _ = server.Write([]byte("/iekEtest\r\n"))
				p0 := []byte{'P', '0', stx, fb, '1', '2', '3', rb, etx}
				_, _ = server.Write([]byte{soh})
				_, _ = server.Write(p0)
				_, _ = server.Write([]byte{bcc(p0)})
				buf := make([]byte, 15)
				_, _ = server.Read(buf)
			},
			wantErr: false,
		},
		{
			name: "pass",
			fields: fields{
				pass: func(arg DataSet) (DataSet, CommandId) {
					return DataSet{Value: "passw0rd"}, CmdP1
				},
				lastActivity: time.Now(),
			},
			fn: func() {
				buf := make([]byte, 15)
				_, _ = server.Read(buf)
				_, _ = server.Write([]byte("/iek6test\r\n"))
				_, _ = server.Read(buf)
				p0 := []byte{'P', '0', stx, fb, '1', '2', '3', rb, etx}
				_, _ = server.Write([]byte{soh})
				_, _ = server.Write(p0)
				_, _ = server.Write([]byte{bcc(p0)})
				_, _ = server.Read(buf)
				_, _ = server.Write([]byte{ack})
			},
			wantErr: false,
		},
		{
			name: "pass mode B",
			fields: fields{
				pass: func(arg DataSet) (DataSet, CommandId) {
					return DataSet{Value: "passw0rd"}, CmdP1
				},
				lastActivity: time.Now(),
			},
			fn: func() {
				_, _ = server.Write([]byte("/iekEtest\r\n"))
				p0 := []byte{'P', '0', stx, fb, '1', '2', '3', rb, etx}
				_, _ = server.Write([]byte{soh})
				_, _ = server.Write(p0)
				_, _ = server.Write([]byte{bcc(p0)})
				_, _ = server.Write([]byte{ack})
			},
			wantErr: false,
		},
		{
			name: "pass nak",
			fields: fields{
				pass: func(arg DataSet) (DataSet, CommandId) {
					return DataSet{Value: "passw0rd"}, CmdP1
				},
				lastActivity: time.Now(),
			},
			fn: func() {
				buf := make([]byte, 15)
				_, _ = server.Read(buf)
				_, _ = server.Write([]byte("/iekEtest\r\n"))
				_, _ = server.Read(buf)
				p0 := []byte{'P', '0', stx, fb, '1', '2', '3', rb, etx}
				_, _ = server.Write([]byte{soh})
				_, _ = server.Write(p0)
				_, _ = server.Write([]byte{bcc(p0)})
				_, _ = server.Read(buf)
				_, _ = server.Write([]byte{nak, nak, nak, nak, nak})
			},
			wantErr: true,
		},
		{
			name: "pass Break",
			fields: fields{
				pass: func(arg DataSet) (DataSet, CommandId) {
					return DataSet{Value: "passw0rd"}, CmdP1
				},
				lastActivity: time.Now(),
			},
			fn: func() {
				buf := make([]byte, 15)
				_, _ = server.Read(buf)
				_, _ = server.Write([]byte("/iekEtest\r\n"))
				_, _ = server.Read(buf)
				p0 := []byte{'P', '0', stx, fb, '1', '2', '3', rb, etx}
				_, _ = server.Write([]byte{soh})
				_, _ = server.Write(p0)
				_, _ = server.Write([]byte{bcc(p0)})
				_, _ = server.Read(buf)
				_, _ = server.Write(breakMsg)
				_, _ = server.Write([]byte{bcc(breakMsg[1:])})
			},
			wantErr: true,
		},
		{
			name: "pass Error",
			fields: fields{
				pass: func(arg DataSet) (DataSet, CommandId) {
					return DataSet{Value: "passw0rd"}, CmdP1
				},
				lastActivity: time.Now(),
			},
			fn: func() {
				buf := make([]byte, 15)
				_, _ = server.Read(buf)
				_, _ = server.Write([]byte("/iekEtest\r\n"))
				_, _ = server.Read(buf)
				p0 := []byte{'P', '0', stx, fb, '1', '2', '3', rb, etx}
				_, _ = server.Write([]byte{soh})
				_, _ = server.Write(p0)
				_, _ = server.Write([]byte{bcc(p0)})
				_, _ = server.Read(buf)
				errMsg := []byte{fb, 'E', 'R', 'R', '1', rb, etx}
				_, _ = server.Write([]byte{stx})
				_, _ = server.Write(errMsg)
				_, _ = server.Write([]byte{bcc(errMsg)})
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := NewTariffDevice(client)
			tr.pass = tt.fields.pass
			tr.lastActivity = tt.fields.lastActivity
			go tt.fn()
			if err := tr.enterProgrammingMode(); (err != nil) != tt.wantErr {
				t.Errorf("TariffDevice.enterProgrammingMode() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTariffDevice_ImmediateReadOut(t *testing.T) {
	server, client := listen()
	defer client.Close()
	defer server.Close()
	tests := []struct {
		name    string
		fn      func()
		want    *DataBlock
		ident   *Identity
		wantErr bool
	}{
		{
			name: "Data readout",
			fn: func() {
				_, _ = server.Write([]byte("/ekt3id\r\n\r\nAddr(Val)!\r\n"))
			},
			want: &DataBlock{
				Lines: []DataLine{
					{
						Sets: []DataSet{
							{
								Address: "Addr",
								Value:   "Val",
							},
						},
					},
				},
			},
			ident: &Identity{
				Device:       "id",
				Manufacturer: "ekt",
				Mode:         ModeD,
				bri:          '3',
			},
			wantErr: false,
		},
		{
			name: "Error frame no lf",
			fn: func() {
				_, _ = server.Write([]byte("/ekt3id\r\n\nAddr(Val)!\r\n"))
			},
			want:    nil,
			ident:   nil,
			wantErr: true,
		},
		{
			name: "Error frame no cr",
			fn: func() {
				_, _ = server.Write([]byte("/ekt3id\r\n\rAddr(Val)!\r\n"))
			},
			want:    nil,
			ident:   nil,
			wantErr: true,
		},
		{
			name: "Error invalid data",
			fn: func() {
				_, _ = server.Write([]byte("/ekt3id\r\n\r\nAddrVal)!\r\n"))
			},
			want:    nil,
			ident:   nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := NewTariffDevice(client)
			go tt.fn()
			ident, got, err := tr.ImmediateDreadOut()
			if (err != nil) != tt.wantErr {
				t.Errorf("TariffDevice.ImmediateReadOut() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TariffDevice.ImmediateReadOut() = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(ident, tt.ident) {
				t.Errorf("TariffDevice.ImmediateReadOut() = %v, want %v", got, tt.ident)
			}
		})
	}
}

func TestTariffDevice_isInProgrammingMode(t *testing.T) {
	conn := getClosedConn()
	type fields struct {
		programmingMode bool
		lastActivity    time.Time
		identity        *Identity
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		{
			name: "No identity",
			fields: fields{
				identity:        nil,
				lastActivity:    time.Now(),
				programmingMode: true,
			},
			want: false,
		},
		{
			name: "Too old",
			fields: fields{
				identity:        &Identity{},
				lastActivity:    time.Now().Add(-(defaultInactivityTo + 5)),
				programmingMode: true,
			},
			want: false,
		},
		{
			name: "False mode value",
			fields: fields{
				identity:        &Identity{},
				lastActivity:    time.Now(),
				programmingMode: false,
			},
			want: false,
		},
		{
			name: "True",
			fields: fields{
				identity:        &Identity{},
				lastActivity:    time.Now(),
				programmingMode: true,
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := NewTariffDevice(conn)
			td.identity = tt.fields.identity
			td.lastActivity = tt.fields.lastActivity
			td.programmingMode = tt.fields.programmingMode
			got := td.isInProgrammingMode()
			if got != tt.want {
				t.Errorf("TariffDevice.isInProgrammingMode() = %v, want %v", got, tt.want)
			}
		})

	}

}

func Test_bcc(t *testing.T) {
	type args struct {
		data []byte
	}
	tests := []struct {
		name string
		args args
		want byte
	}{
		{
			name: "set #1",
			args: args{
				data: []byte{1, 2, 3, 4, 5, 6, 7, 8},
			},
			want: 36,
		},
		{
			name: "set #2",
			args: args{
				data: []byte("/xxx3yyy\r\n"),
			},
			want: 76,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := bcc(tt.args.data); got != tt.want {
				t.Errorf("bcc() = %v, want %v", got, tt.want)
			}
		})
	}
}
