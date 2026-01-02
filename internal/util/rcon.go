package util

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"strings"
	"sync/atomic"
	"time"

	"github.com/validator-gcp/v2/internal/apperror"
)

const (
	PKT_AUTH               = 3
	PKT_COMMAND            = 2
	PKT_RESPONSE_VALUE     = 0
	PKT_AUTH_RESPONSE      = 2 // Auth response also uses type 2 usually
	PKT_INVALID_AUTH       = -1
	DUMMY_CMD_GENERATED_ID = 200
)

type RconPacket struct {
	RequestID int32
	Type      int32
	Body      string
}

var rconIdCounter int32 = int32(rand.Intn(10000))

func getNextRconID() int32 {
	return atomic.AddInt32(&rconIdCounter, 1)
}

func writeRconPacket(w io.Writer, reqID int32, typ int32, payload string) error {
	payloadBytes := []byte(payload)
	payloadBytes = append(payloadBytes, 0x00) // null terminator

	// Length = ID (4) + Type (4) + Body (n) + Null (1)
	// The Packet Length field itself is NOT included in the size calculation in RCON standard,
	// but the int32 field FOR the length is sent before everything.
	packetLen := int32(4 + 4 + len(payloadBytes) + 1)

	// Buffer Construction (Little Endian)
	buf := new(bytes.Buffer)

	binary.Write(buf, binary.LittleEndian, packetLen)
	binary.Write(buf, binary.LittleEndian, reqID)
	binary.Write(buf, binary.LittleEndian, typ)

	buf.Write(payloadBytes)
	buf.Write([]byte{0x00})

	_, err := w.Write(buf.Bytes())

	if typ != PKT_AUTH {
		log.Printf("[RCON] write: %v with reqId=%d", payload, reqID)
	}
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

	// Since we create a new connection per request, static IDs are fine.
	REQUEST_GENERATED_ID := getNextRconID()
	COMMAND_GENERATED_ID := getNextRconID()

	/*
		SEND: REQUEST_GENERATED_ID: atomic int | TYPE: 3 | PASSWORD | PADDING
	*/

	err = writeRconPacket(conn, REQUEST_GENERATED_ID, PKT_AUTH, password)
	if err != nil {
		return "", apperror.ErrInternal
	}

	// read result
	authResp, err := readRconPacket(conn)
	if err != nil {
		return "", apperror.ErrInternal
	}

	// Basic checks
	if authResp.RequestID == PKT_INVALID_AUTH {
		log.Printf("[RCON]: password seems incorrect")
		return "", apperror.ErrInternal
	}
	if authResp.RequestID != REQUEST_GENERATED_ID {
		log.Printf("[RCON]: auth protocol error: mismatched ids: got %d expected %d", authResp.RequestID, REQUEST_GENERATED_ID)
		return "", apperror.ErrInternal
	}

	/*
		At this point, we are authenticated successfully

		SEND #1: ID | TYPE: 2 | COMMAND | PADDING
		SEND #2: NEW ID | TYPE : 200 (invalid) | DUMMY CMD | PADDING

		An alternative is for the second C->S packet to use an invalid type (say, 200); the server will respond
		with a 'Command response' packet with its payload set to 'Unknown request c8'. (200 in hexadecimal)
	*/
	err = writeRconPacket(conn, COMMAND_GENERATED_ID, PKT_COMMAND, command)
	if err != nil {
		return "", err
	}

	/*
		IMPORTANT: A tiny sleep here forces the OS to flush the previous packet
		before we queue the next one. WIthout this, the the whole flow breaks like
		50% of the time - with no patterns in commands. its just EOF.

		My theory is the issue isnt gone, its just a hack for things to complete.
		Anyways if its the minecraft server's issue I cant do much here. Ill try to
		research more and see if we can do it better. 2ms isnt that much anyways.
	*/
	time.Sleep(2 * time.Millisecond)

	err = writeRconPacket(conn, DUMMY_CMD_GENERATED_ID, PKT_COMMAND, "hit_the_griddy")
	if err != nil {
		return "", err
	}
	// reading the responses we get.

	var responseBody strings.Builder

	for {
		pkt, err := readRconPacket(conn)
		if err != nil {
			return "", apperror.ErrInternal
		}

		if pkt.RequestID == COMMAND_GENERATED_ID {
			// Part of our command response
			responseBody.WriteString(pkt.Body)

		} else if pkt.RequestID == DUMMY_CMD_GENERATED_ID {
			// Sentinel received! We are done.
			break

		} else {
			// Unexpected packet (Keep-alives? Chat messages? Protocol violation?)
			log.Printf("[RCON]: received unexpected packet ID :: expected cmd=%v sentinel=%v, got=%v (type=%d, body_len=%d)", COMMAND_GENERATED_ID, DUMMY_CMD_GENERATED_ID, pkt.RequestID, pkt.Type, len(pkt.Body))
			// In strict mode we might throw error, but usually we just ignore or log
			continue
		}
	}

	return responseBody.String(), nil
}
