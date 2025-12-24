package protocol

import (
	"encoding/binary"
	"hash/crc32"
	"testing"
)

func TestBuildAndParseV4(t *testing.T) {
	req, err := BuildRequest("check_load", []string{"arg1", "arg2"})
	if err != nil {
		t.Fatalf("BuildRequest failed: %v", err)
	}
	resp := make([]byte, len(req))
	copy(resp, req)
	binary.BigEndian.PutUint16(resp[2:4], packetTypeResponse)
	binary.BigEndian.PutUint32(resp[4:8], 0)
	crc := checksum(resp)
	binary.BigEndian.PutUint32(resp[4:8], crc)
	pkt, err := ParseResponse(resp)
	if err != nil {
		t.Fatalf("ParseResponse failed: %v", err)
	}
	if PayloadString(pkt) != "check_load!arg1!arg2" {
		t.Fatalf("unexpected payload: %q", PayloadString(pkt))
	}
}

func TestBuildRequestLayout(t *testing.T) {
	req, err := BuildRequest("hello", []string{"world"})
	if err != nil {
		t.Fatalf("BuildRequest failed: %v", err)
	}
	if len(req) != MinPacketLength() {
		t.Fatalf("unexpected packet len %d", len(req))
	}
	if got := binary.BigEndian.Uint16(req[0:2]); got != Version4 {
		t.Fatalf("unexpected version %d", got)
	}
	if got := binary.BigEndian.Uint16(req[2:4]); got != packetTypeQuery {
		t.Fatalf("unexpected type %d", got)
	}
	bufLen := int(binary.BigEndian.Uint32(req[12:16]))
	if bufLen != MinPacketLength()-HeaderLength() {
		t.Fatalf("unexpected buffer len %d", bufLen)
	}
	payload := req[HeaderLength() : HeaderLength()+len("hello!world")+1]
	if string(payload[:len(payload)-1]) != "hello!world" {
		t.Fatalf("unexpected payload %q", payload)
	}
	crcField := binary.BigEndian.Uint32(req[4:8])
	copyBuf := make([]byte, len(req))
	copy(copyBuf, req)
	binary.BigEndian.PutUint32(copyBuf[4:8], 0)
	if want := crc32.ChecksumIEEE(copyBuf); want != crcField {
		t.Fatalf("crc mismatch want=%x got=%x", want, crcField)
	}
}

func TestBuildRequestRejectsEmpty(t *testing.T) {
	if _, err := BuildRequest("", nil); err == nil {
		t.Fatalf("expected error for empty command")
	}
}

func TestParseResponseDetectsBadCRC(t *testing.T) {
	req, err := BuildRequest("cmd", nil)
	if err != nil {
		t.Fatalf("BuildRequest failed: %v", err)
	}
	resp := make([]byte, len(req))
	copy(resp, req)
	binary.BigEndian.PutUint16(resp[2:4], packetTypeResponse)
	binary.BigEndian.PutUint16(resp[8:10], uint16(StatusCritical))
	crc := binary.BigEndian.Uint32(resp[4:8])
	binary.BigEndian.PutUint32(resp[4:8], crc+1)
	if _, err := ParseResponse(resp); err == nil {
		t.Fatalf("expected CRC error")
	}
}
