package util

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"strings"

	"github.com/validator-gcp/v2/internal/apperror"
)

const (
	PKT_AUTH           = 3
	PKT_COMMAND        = 2
	PKT_RESPONSE_VALUE = 0
	PKT_AUTH_RESPONSE  = 2 // Auth response also uses type 2 usually
	PKT_INVALID_AUTH   = -1
)

type RconPacket struct {
	RequestID int32
	Type      int32
	Body      string
}

func writeRconPacket(w io.Writer, reqID int32, typ int32, payload string) error {
	payloadBytes := []byte(payload)

	// Length = ID (4) + Type (4) + Body (n) + Null (1) + Null (1)
	// The Packet Length field itself is NOT included in the size calculation in RCON standard,
	// but the int32 field FOR the length is sent before everything.
	packetLen := int32(4 + 4 + len(payloadBytes) + 1 + 1)

	// Buffer Construction (Little Endian)
	buf := new(bytes.Buffer)

	binary.Write(buf, binary.LittleEndian, packetLen)
	binary.Write(buf, binary.LittleEndian, reqID)
	binary.Write(buf, binary.LittleEndian, typ)

	buf.Write(payloadBytes)
	buf.Write([]byte{0x00, 0x00})

	_, err := w.Write(buf.Bytes())
	return err
}

func readRconPacket(r io.Reader) (*RconPacket, error) {
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(r, lenBuf); err != nil {
		return nil, fmt.Errorf("failed to read packet length: %w", err)
	}

	packetLen := int32(binary.LittleEndian.Uint32(lenBuf))

	// We use ReadFull because TCP can fragment packets, and we need exact bytes.
	dataBuf := make([]byte, packetLen)
	if _, err := io.ReadFull(r, dataBuf); err != nil {
		return nil, fmt.Errorf("failed to read packet body: %w", err)
	}

	// Data Structure: [ID 4] [Type 4] [Body ... ] [Null] [Null]

	// Create a reader for the data buffer to make parsing easy
	dataReader := bytes.NewReader(dataBuf)

	var reqID int32
	var typ int32
	binary.Read(dataReader, binary.LittleEndian, &reqID)
	binary.Read(dataReader, binary.LittleEndian, &typ)

	// Header size inside dataBuf is 8 bytes (ID+Type).
	// So Body is dataBuf[8 : len-2]
	if len(dataBuf) < 10 { // 8 header + 2 nulls min
		return &RconPacket{RequestID: reqID, Type: typ, Body: ""}, nil
	}

	bodyBytes := dataBuf[8 : len(dataBuf)-2]
	return &RconPacket{
		RequestID: reqID,
		Type:      typ,
		Body:      string(bodyBytes),
	}, nil
}

/*
Executes the given command, though many commands return nothing,
and there's no way of detecting unknown commands.
*/
func ExecuteCommand(ctx context.Context, command string, host string, port int, password string) (string, error) {
	address := fmt.Sprintf("%s:%d", host, port)

	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", address)
	if err != nil {
		log.Printf("rcon dial failed: %v", err)
		return "", apperror.ErrInternal
	}
	defer conn.Close()

	if deadline, ok := ctx.Deadline(); ok {
		conn.SetDeadline(deadline)
	}

	// Generate Random IDs (or just incrementing ones)
	// Simple approach: use hardcoded offsets or rand.
	// Since we create a new connection per request, static IDs are fine.
	const loginID = 1
	const cmdID = 2
	const sentinelID = 3

	// --- AUTHENTICATION ---

	// Send Login
	err = writeRconPacket(conn, loginID, PKT_AUTH, password)
	if err != nil {
		return "", apperror.ErrInternal
	}

	// Read Login Response
	authResp, err := readRconPacket(conn)
	if err != nil {
		return "", apperror.ErrInternal
	}

	// Check Auth
	if authResp.RequestID == PKT_INVALID_AUTH {
		log.Printf("[RCON]: password seems incorrect")
		return "", apperror.ErrInternal
	}
	if authResp.RequestID != loginID {
		log.Printf("[RCON]: auth protocol error: mismatched ids: got %d expected %d", authResp.RequestID, loginID)
		return "", apperror.ErrInternal
	}

	// --- EXECUTION ---

	err = writeRconPacket(conn, cmdID, PKT_COMMAND, command)
	if err != nil {
		return "", err
	}

	err = writeRconPacket(conn, sentinelID, PKT_RESPONSE_VALUE, "")
	if err != nil {
		return "", err
	}

	// --- READ LOOP ---

	var responseBody strings.Builder

	for {
		pkt, err := readRconPacket(conn)
		if err != nil {
			log.Printf("[RCON]: stream error: %v", err)
			return "", apperror.ErrInternal
		}

		if pkt.RequestID == cmdID {
			// Part of our command response
			responseBody.WriteString(pkt.Body)

		} else if pkt.RequestID == sentinelID {
			// Sentinel received! We are done.
			break

		} else {
			// Unexpected packet (Keep-alives? Chat messages? Protocol violation?)
			log.Printf("[RCON]: received unexpected packet ID :: expected: %v, got: %v\n", cmdID, pkt.RequestID)
			// In strict mode we might throw error, but usually we just ignore or log
			continue
		}
	}

	return responseBody.String(), nil
}
