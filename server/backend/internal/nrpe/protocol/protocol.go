package protocol

import (
	"encoding/binary"
	"errors"
	"strings"
)

const (
	Version4           = 4
	packetTypeQuery    = 1
	packetTypeResponse = 2
	headerLength       = 16
	minPacketLength    = 1036
	maxPayloadLength   = 65535
)

type Status uint16

const (
	StatusOK Status = iota
	StatusWarning
	StatusCritical
	StatusUnknown
)

type Packet struct {
	Version uint16
	Type    uint16
	Result  Status
	Payload []byte
}

var crcTable [256]uint32

func init() {
	const poly = 0xEDB88320
	for i := 0; i < 256; i++ {
		crc := uint32(i)
		for j := 0; j < 8; j++ {
			if crc&1 == 1 {
				crc = (crc >> 1) ^ poly
			} else {
				crc >>= 1
			}
		}
		crcTable[i] = crc
	}
}

func BuildRequest(command string, args []string) ([]byte, error) {
	payload := []byte(buildPayloadString(command, args))
	if len(payload) == 0 {
		return nil, errors.New("nrpe: empty command")
	}
	if len(payload) >= maxPayloadLength {
		return nil, errors.New("nrpe payload too large")
	}
	payload = append(payload, 0x00)
	packetLen := headerLength + len(payload)
	if packetLen < minPacketLength {
		packetLen = minPacketLength
	}
	buf := make([]byte, packetLen)
	binary.BigEndian.PutUint16(buf[0:2], Version4)
	binary.BigEndian.PutUint16(buf[2:4], packetTypeQuery)
	binary.BigEndian.PutUint32(buf[4:8], 0)
	binary.BigEndian.PutUint16(buf[8:10], uint16(StatusOK))
	binary.BigEndian.PutUint16(buf[10:12], 0)
	bufferLen := packetLen - headerLength
	binary.BigEndian.PutUint32(buf[12:16], uint32(bufferLen))
	copy(buf[headerLength:], payload)
	crc := checksum(buf)
	binary.BigEndian.PutUint32(buf[4:8], crc)
	return buf, nil
}

func ParseResponse(raw []byte) (Packet, error) {
	if len(raw) < headerLength {
		return Packet{}, errors.New("packet too small")
	}
	if err := verifyCRC(raw); err != nil {
		return Packet{}, err
	}
	pkt := Packet{}
	pkt.Version = binary.BigEndian.Uint16(raw[0:2])
	if pkt.Version != Version4 {
		return Packet{}, errors.New("unexpected version")
	}
	pkt.Type = binary.BigEndian.Uint16(raw[2:4])
	if pkt.Type != packetTypeResponse {
		return Packet{}, errors.New("unexpected packet type")
	}
	pkt.Result = Status(binary.BigEndian.Uint16(raw[8:10]))
	bufferLen := int(binary.BigEndian.Uint32(raw[12:16]))
	if bufferLen < 0 {
		return Packet{}, errors.New("invalid buffer length")
	}
	payloadStart := headerLength
	if payloadStart > len(raw) {
		return Packet{}, errors.New("truncated packet")
	}
	available := len(raw) - payloadStart
	if bufferLen > available {
		bufferLen = available
	}
	payload := raw[payloadStart : payloadStart+bufferLen]
	end := len(payload)
	for i, b := range payload {
		if b == 0 {
			end = i
			break
		}
	}
	pkt.Payload = make([]byte, end)
	copy(pkt.Payload, payload[:end])
	return pkt, nil
}

func PayloadString(p Packet) string {
	return strings.TrimRight(string(p.Payload), "\x00")
}

func verifyCRC(data []byte) error {
	expected := binary.BigEndian.Uint32(data[4:8])
	binary.BigEndian.PutUint32(data[4:8], 0)
	actual := checksum(data)
	binary.BigEndian.PutUint32(data[4:8], expected)
	if expected != actual {
		return errors.New("invalid crc")
	}
	return nil
}

func checksum(data []byte) uint32 {
	crc := uint32(0xFFFFFFFF)
	for _, b := range data {
		crc = (crc >> 8) ^ crcTable[(crc^uint32(b))&0xFF]
	}
	return crc ^ 0xFFFFFFFF
}

func buildPayloadString(command string, args []string) string {
	if len(args) == 0 {
		return command
	}
	return command + "!" + strings.Join(args, "!")
}

func HeaderLength() int { return headerLength }

func MinPacketLength() int { return minPacketLength }
