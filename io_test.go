package iec62056

import (
	"net"
	"reflect"
	"testing"
)

func listenWithParity() (net.Conn, Conn) {
	go func() {
		rv, _ := listener.Accept()
		ch <- rv
	}()
	d := &TCPDialer{
		SwParity: true,
	}
	conn, _ := d.Dial(listener.Addr().String())
	return <-ch, conn
}
func Test_FixParity(t *testing.T) {
	server, client := listenWithParity()
	defer server.Close()
	defer client.Close()

	td := NewTariffDevice(client)
	go func() {
		buf := make([]byte, 10)
		server.Read(buf)
		resp := []byte("/ABC6dev\r\n")
		for i := 0; i < len(resp); i++ {
			resp[i] |= 0x80
		}
		server.Write(resp)
		if !reflect.DeepEqual(buf[:5], []byte{175, 63, 33, 141, 10}) {
			t.Error("Parity encoding failed")
		}
	}()
	res, err := td.Identity()
	if err != nil {
		t.Error(err.Error())
	}

	want := Identity{
		Device:       "dev",
		Manufacturer: "ABC",
		Mode:         ModeC,
		bri:          '6',
	}
	if !reflect.DeepEqual(res, want) {
		t.Errorf("Sw Parity failed. wanted %v, got %v", want, res)
	}

}
