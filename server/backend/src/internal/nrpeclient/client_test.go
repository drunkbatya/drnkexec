package nrpeclient

import (
	"encoding/binary"
	"hash/crc32"
	"net"
	"testing"

	"github.com/drunkbatya/drnkexec/internal/nrpe/protocol"
)

func TestReadResponseHandlesShortBuffer(t *testing.T) {
	c := &client{}
	server, clientConn := net.Pipe()
	defer server.Close()
	defer clientConn.Close()

	go func() {
		packet := buildResponsePacket(protocol.StatusOK, "hello world")
		server.Write(packet)
		server.Close()
	}()

	pkt, err := c.readResponse(clientConn)
	if err != nil {
		t.Fatalf("readResponse failed: %v", err)
	}
	if got := protocol.PayloadString(pkt); got != "hello world" {
		t.Fatalf("unexpected payload %q", got)
	}
	if pkt.Result != protocol.StatusOK {
		t.Fatalf("unexpected status %d", pkt.Result)
	}
}

func TestReadResponseRejectsInvalidCRC(t *testing.T) {
	c := &client{}
	server, clientConn := net.Pipe()
	defer server.Close()
	defer clientConn.Close()

	go func() {
		packet := buildResponsePacket(protocol.StatusOK, "bad")
		packet[4] ^= 0xff
		server.Write(packet)
		server.Close()
	}()

	if _, err := c.readResponse(clientConn); err == nil {
		t.Fatalf("expected error")
	}
}

func buildResponsePacket(status protocol.Status, output string) []byte {
	payload := append([]byte(output), 0x00)
	totalLen := protocol.HeaderLength() + len(payload)
	if totalLen < protocol.MinPacketLength() {
		totalLen = protocol.MinPacketLength()
	}
	buf := make([]byte, totalLen)
	binary.BigEndian.PutUint16(buf[0:2], protocol.Version4)
	binary.BigEndian.PutUint16(buf[2:4], 2)
	binary.BigEndian.PutUint32(buf[4:8], 0)
	binary.BigEndian.PutUint16(buf[8:10], uint16(status))
	binary.BigEndian.PutUint16(buf[10:12], 0)
	bufferLen := totalLen - protocol.HeaderLength()
	binary.BigEndian.PutUint32(buf[12:16], uint32(bufferLen))
	copy(buf[protocol.HeaderLength():], payload)
	crc := crc32.ChecksumIEEE(buf)
	binary.BigEndian.PutUint32(buf[4:8], crc)
	return buf
}
