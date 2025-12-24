package protocol

import (
    "encoding/binary"
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
