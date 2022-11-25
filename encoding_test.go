package iec62056

import (
	"reflect"
	"testing"
)

func TestDataSet_MarshalBinary(t *testing.T) {
	type fields struct {
		Address string
		Value   string
		Unit    string
	}
	tests := []struct {
		name    string
		fields  fields
		want    []byte
		wantErr bool
	}{
		{
			name:    "Empty",
			fields:  fields{},
			want:    []byte{},
			wantErr: false,
		},
		{
			name:    "Just Address",
			fields:  fields{Address: "ADDR"},
			want:    []byte{'A', 'D', 'D', 'R', fb, rb},
			wantErr: false,
		},
		{
			name: "Address and value",
			fields: fields{
				Address: "ARDR",
				Value:   "VAL",
			},
			want:    []byte{'A', 'R', 'D', 'R', fb, 'V', 'A', 'L', rb},
			wantErr: false,
		},
		{
			name: "All filled",
			fields: fields{
				Address: "ADDR",
				Value:   "VLL",
				Unit:    "UN",
			},
			want:    []byte{'A', 'D', 'D', 'R', fb, 'V', 'L', 'L', star, 'U', 'N', rb},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ds := &DataSet{
				Address: tt.fields.Address,
				Value:   tt.fields.Value,
				Unit:    tt.fields.Unit,
			}
			got, err := ds.MarshalBinary()
			if (err != nil) != tt.wantErr {
				t.Errorf("DataSet.MarshalBinary() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DataSet.MarshalBinary() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDataSet_UnmarshalBinary(t *testing.T) {
	type fields struct {
		Address string
		Value   string
		Unit    string
	}
	type args struct {
		data []byte
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "Just addr",
			fields: fields{
				Address: "ADDR",
			},
			args: args{
				data: []byte{'A', 'D', 'D', 'R', fb, rb},
			},
			wantErr: false,
		},
		{
			name: "Addr and value",
			fields: fields{
				Address: "ARDR",
				Value:   "VAL",
			},
			args: args{
				data: []byte{'A', 'R', 'D', 'R', fb, 'V', 'A', 'L', rb},
			},
			wantErr: false,
		},
		{
			name: "All",
			fields: fields{
				Address: "ADDR",
				Value:   "VLL",
				Unit:    "UN",
			},
			args: args{
				data: []byte{'A', 'D', 'D', 'R', fb, 'V', 'L', 'L', star, 'U', 'N', rb},
			},
			wantErr: false,
		},
		{
			name: "No front",
			fields: fields{
				Address: "ADDR",
			},
			args: args{
				data: []byte{'A', 'D', 'D', 'R', rb},
			},
			wantErr: true,
		},
		{
			name: "No Rear",
			fields: fields{
				Address: "",
			},
			args: args{
				data: []byte{fb},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ds := &DataSet{
				Address: tt.fields.Address,
				Value:   tt.fields.Value,
				Unit:    tt.fields.Unit,
			}
			if err := ds.UnmarshalBinary(tt.args.data); (err != nil) != tt.wantErr {
				t.Errorf("DataSet.UnmarshalBinary() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDataLine_UnmarshalBinary(t *testing.T) {
	type fields struct {
		Sets []DataSet
	}
	type args struct {
		data []byte
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name:    "Empty",
			fields:  fields{},
			args:    args{},
			wantErr: false,
		},
		{
			name: "One line",
			fields: fields{
				Sets: []DataSet{{Address: "ADDR"}},
			},
			args: args{
				[]byte{'A', 'D', 'D', 'R', fb, rb},
			},
			wantErr: false,
		},
		{
			name: "Multi line",
			fields: fields{
				Sets: []DataSet{{Address: "ADDR"}, {Address: "ALDR", Value: "VAL"}},
			},
			args: args{
				[]byte{'A', 'D', 'D', 'R', fb, rb, 'A', 'L', 'D', 'R', fb, 'V', 'A', 'L', rb},
			},
			wantErr: false,
		},
		{
			name: "Error in line",
			fields: fields{
				Sets: []DataSet{{Address: "ADDR"}},
			},
			args: args{
				[]byte{'A', 'D', 'D', 'R', fb},
			},
			wantErr: true,
		},
		{
			name: "Error in line no fb",
			fields: fields{
				Sets: []DataSet{{Address: "ADDR"}},
			},
			args: args{
				[]byte{'A', 'D', 'D', 'R', rb},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dl := &DataLine{
				Sets: tt.fields.Sets,
			}
			if err := dl.UnmarshalBinary(tt.args.data); (err != nil) != tt.wantErr {
				t.Errorf("DataLine.UnmarshalBinary() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDataBlock_UnmarshalBinary(t *testing.T) {
	type fields struct {
		Lines []DataLine
	}
	type args struct {
		data []byte
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name:    "Empty",
			fields:  fields{},
			args:    args{},
			wantErr: false,
		},
		{
			name: "One line",
			fields: fields{
				Lines: []DataLine{{
					Sets: []DataSet{{Address: "ADDR"}},
				}},
			},
			args: args{
				[]byte{'A', 'D', 'D', 'R', fb, rb},
			},
			wantErr: false,
		},
		{
			name: "Multi line",
			fields: fields{
				Lines: []DataLine{
					{
						Sets: []DataSet{{Address: "ADDR"}},
					},
					{
						Sets: []DataSet{{Address: "ALDR"}},
					},
				},
			},
			args: args{
				[]byte{'A', 'D', 'D', 'R', fb, rb, cr, lf, 'A', 'L', 'D', 'R', fb, 'V', 'A', 'L', rb},
			},
			wantErr: false,
		},
		{
			name: "Error in data set§",
			fields: fields{
				Lines: []DataLine{{
					Sets: []DataSet{{Address: "ADDR"}},
				}},
			},
			args: args{
				[]byte{'A', 'D', 'D', 'R', rb},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := &DataBlock{
				Lines: tt.fields.Lines,
			}
			if err := db.UnmarshalBinary(tt.args.data); (err != nil) != tt.wantErr {
				t.Errorf("DataBlock.UnmarshalBinary() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCommand_MarshalBinary(t *testing.T) {
	type fields struct {
		Id      CommandId
		Payload *DataSet
	}
	tests := []struct {
		name    string
		fields  fields
		want    []byte
		wantErr bool
	}{
		{
			name: "No Payload",
			fields: fields{
				Id: CmdB0,
			},
			want:    []byte{soh, 'B', '0', etx},
			wantErr: false,
		},
		{
			name: "With Payload",
			fields: fields{
				Id:      CmdR1,
				Payload: &DataSet{Address: "ADDR"},
			},
			want:    []byte{soh, 'R', '1', stx, 'A', 'D', 'D', 'R', fb, rb, etx},
			wantErr: false,
		},
		{
			name: "Unknown Command",
			fields: fields{
				Id: 45,
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Command{
				Id:      tt.fields.Id,
				Payload: tt.fields.Payload,
			}
			got, err := c.MarshalBinary()
			if (err != nil) != tt.wantErr {
				t.Errorf("Command.MarshalBinary() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Command.MarshalBinary() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOptionSelectMessage_MarshalBinary(t *testing.T) {
	type fields struct {
		Option        Option
		PCC           PCC
		bri           byte
		skipHandShake bool
	}
	tests := []struct {
		name    string
		fields  fields
		want    []byte
		wantErr bool
	}{
		{
			name: "ReadOut",
			fields: fields{
				Option:        DataReadOut,
				PCC:           NormalPCC,
				bri:           '5',
				skipHandShake: false,
			},
			want:    []byte{ack, byte(NormalPCC), '5', byte(DataReadOut)},
			wantErr: false,
		},
		{
			name: "Programming mode",
			fields: fields{
				Option:        ProgrammingMode,
				PCC:           SecondaryPCC,
				bri:           'A',
				skipHandShake: false,
			},
			want:    []byte{ack, byte(SecondaryPCC), 'A', byte(ProgrammingMode)},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := &OptionSelectMessage{
				Option:        tt.fields.Option,
				PCC:           tt.fields.PCC,
				bri:           tt.fields.bri,
				skipHandShake: tt.fields.skipHandShake,
			}
			got, err := o.MarshalBinary()
			if (err != nil) != tt.wantErr {
				t.Errorf("OptionSelectMessage.MarshalBinary() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("OptionSelectMessage.MarshalBinary() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_requestMessage_MarshalBinary(t *testing.T) {
	tests := []struct {
		name    string
		address requestMessage
		want    []byte
		wantErr bool
	}{
		{
			name:    "Broadcast",
			address: "",
			want:    []byte{start, trc, end},
			wantErr: false,
		},
		{
			name:    "Named",
			address: "addr",
			want:    []byte{start, trc, 'a', 'd', 'd', 'r', end},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.address.MarshalBinary()
			if (err != nil) != tt.wantErr {
				t.Errorf("requestMessage.MarshalBinary() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("requestMessage.MarshalBinary() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIdentity_UnmarshalBinary(t *testing.T) {
	type fields struct {
		Device       string
		Manufacturer string
		Mode         ProtocolMode
		bri          byte
	}
	type args struct {
		data []byte
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name:    "Error too short",
			fields:  fields{},
			args:    args{[]byte{1, 2, 3}},
			wantErr: true,
		},
		{
			name: "Parse OK",
			fields: fields{
				"iek",
				"test",
				ModeC,
				'E',
			},
			args:    args{[]byte("iekEtest")},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := &Identity{
				Device:       tt.fields.Device,
				Manufacturer: tt.fields.Manufacturer,
				Mode:         tt.fields.Mode,
				bri:          tt.fields.bri,
			}
			if err := id.UnmarshalBinary(tt.args.data); (err != nil) != tt.wantErr {
				t.Errorf("Identity.UnmarshalBinary() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_decodeMode(t *testing.T) {
	type args struct {
		b byte
	}
	tests := []struct {
		name string
		args args
		want ProtocolMode
	}{
		{
			name: "Mode A",
			args: args{'x'},
			want: ModeA,
		},
		{
			name: "Mode B (A)",
			args: args{'A'},
			want: ModeB,
		},
		{
			name: "Mode B (D)",
			args: args{'D'},
			want: ModeB,
		},
		{
			name: "Mode B (I)",
			args: args{'I'},
			want: ModeB,
		},
		{
			name: "Mode C (0)",
			args: args{'0'},
			want: ModeC,
		},
		{
			name: "Mode C (2)",
			args: args{'2'},
			want: ModeC,
		},
		{
			name: "Mode C (9)",
			args: args{'9'},
			want: ModeC,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := decodeMode(tt.args.b); got != tt.want {
				t.Errorf("decodeMode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_decodeBaudRate(t *testing.T) {
	type args struct {
		b byte
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		{
			name: "600",
			args: args{'A'},
			want: 600,
		},
		{
			name: "600",
			args: args{'1'},
			want: 600,
		},
		{
			name: "1200",
			args: args{'B'},
			want: 1200,
		},
		{
			name: "1200",
			args: args{'2'},
			want: 1200,
		},
		{
			name: "2400",
			args: args{'C'},
			want: 2400,
		},
		{
			name: "2400",
			args: args{'3'},
			want: 2400,
		},
		{
			name: "4800",
			args: args{'D'},
			want: 4800,
		},
		{
			name: "4800",
			args: args{'4'},
			want: 4800,
		},
		{
			name: "9600",
			args: args{'E'},
			want: 9600,
		},
		{
			name: "9600",
			args: args{'5'},
			want: 9600,
		},
		{
			name: "default",
			args: args{'x'},
			want: 300,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := decodeBaudRate(tt.args.b); got != tt.want {
				t.Errorf("decodeBaudRate() = %v, want %v", got, tt.want)
			}
		})
	}
}
