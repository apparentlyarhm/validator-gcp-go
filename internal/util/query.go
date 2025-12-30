package util

import (
	"bytes"
	"encoding/binary"
	"log"
	"slices"
	"strconv"
	"strings"

	"github.com/validator-gcp/v2/internal/apperror"
	"github.com/validator-gcp/v2/internal/models"
)

/*
first two are MAGIC
next element is TYPE: 09 for handshake and 00 for stat
*/
var HANDSHAKE_PKT = []byte{0xFE, 0xFD, 0x09}

/*
Can be used as a identifier/validator for responses
*/
var SESSION_ID = []byte{0x01, 0x02, 0x03, 0x04}

/*
same as HANDSHAKE_PKT but TYPE is 00.
*/
var STAT_PKT = []byte{0xFE, 0xFD, 0x00}

/*
Used in conjunction with Challenge token for FULL STAT
*/
var PADDING = []byte{0x00, 0x00, 0x00, 0x00}

// Validates first response packet by comparing w TYPE and MAGIC, returns int32 value of the extracted challenge token string
func ValidateAndGetChallengeToken(res []byte, n int) (int32, error) {
	var ct string
	/*
		while responding to the query request is as follows:
		1. first 2 bytes => magic - always FE, FD
		2. next is TYPE (handshake(2))
		3. then we have the session id.
	*/
	if n < 7 {
		return 0, apperror.ErrInternal
	}

	if res[0] != HANDSHAKE_PKT[2] && !slices.Equal(res[1:5], SESSION_ID) {
		return 0, apperror.ErrInternal
	}

	ct = string(bytes.TrimSpace(res[5 : n-1]))
	tokenInt, _ := strconv.Atoi(ct)

	return int32(tokenInt), nil
}

// Incompletely validates/parses the second packet and returns response struct
func ParseStatResponse(res []byte, n int) (*models.MOTDResponse, error) {
	/*
		Structure: [Type 00] [Session 4]
		PAYLOAD : [Padding 11 bytes] [KV Section] [Padding] [Players]
		Total Header Size = 1 + 4 + 11 = 16 bytes
	*/

	/*
		A simple way to parse the payload is to ignore the first 11 bytes, and then split the
		response around the token \x00\x01player_\x00\x00. At the very end, there's an extra null byte.
	*/
	if n < 16 {
		log.Printf("[QUERY] second packet response is too short %v : %v", n, string(res))
		return nil, apperror.ErrInternal
	}

	cursor := 16
	data := res[cursor:n]

	sections := strings.Split(string(data), "\x00\x01player_\x00\x00")
	if len(sections) < 2 {
		log.Printf("[QUERY] Unexpected format in full stat response :: %v", string(data))
		return nil, apperror.ErrInternal
	}

	info := make(map[string]string)

	// if we do var players []string, == nil is true and JSON encoding becomes null
	players := []string{}

	kvPairs := strings.Split(sections[0], "\x00")
	for i := 0; i < len(kvPairs)-1; i += 2 {
		key := kvPairs[i]
		val := kvPairs[i+1]
		if key == "" {
			continue
		}
		info[key] = val
	}

	playerRawResponse := strings.SplitSeq(sections[1], "\x00")
	for item := range playerRawResponse {
		if len(item) != 0 {
			players = append(players, item)
		}
	}

	port, _ := strconv.Atoi(info["hostport"])
	maxPlayers, _ := strconv.Atoi(info["maxplayers"])

	return &models.MOTDResponse{
		Hostname:     info["hostname"],
		GameType:     info["gametype"],
		GameId:       info["game_id"],
		Version:      info["version"],
		Map:          info["map"],
		Players:      players,
		PlayerNumber: len(players),
		MaxPlayers:   maxPlayers,
		HostPort:     port,
	}, nil
}

// Helper to construct the single atomic UDP packet
func CreateStatPacket(token int32) []byte {
	buf := new(bytes.Buffer)
	buf.Write(STAT_PKT)
	buf.Write(SESSION_ID)
	binary.Write(buf, binary.BigEndian, token)
	buf.Write(PADDING)

	return buf.Bytes()
}
