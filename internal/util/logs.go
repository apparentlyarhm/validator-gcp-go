package util

import (
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/validator-gcp/v2/internal/apperror"
	"github.com/validator-gcp/v2/internal/config"
	"github.com/validator-gcp/v2/internal/models"
	"golang.org/x/crypto/ssh"
)

var logRegex = regexp.MustCompile(`^\[([^\]]+)\]\s+\[([^\]]+)\]\s+\[([^\]]+)\]:?\s+(.*)`)

const MAX_MSG_LENGTH = 100
const RCON_STRING = "Thread RCON Client" // this is just noise

type LogResponse struct {
	Timestamp time.Time `json:"timestamp"`
	Lines     []string  `json:"lines"`
}

// a struct for all regexes to apply and what to replace a match with.
// currently redaction will be applied to only messages, not timestamps, src, etc.
type RedactionRule struct {
	Pattern     *regexp.Regexp
	Replacement string
}

var redactionRules = []RedactionRule{

	{
		Pattern:     regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b\b:\d{2,5}\b`),
		Replacement: "<< HOST:PORT >>",
	},

	// might add more.
}

/*
connects to the vm via ssh and fetches recent logs.

All errors that occur here are internal.
*/
func FetchLogs(cfg *config.SSHConfig, lineCount int, add string) (*[]models.LogItem, error) {
	key, e := config.GetPrivateKey(cfg.PKeyB64)
	if e != nil {
		return nil, apperror.ErrInternal
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, apperror.ErrInternal
	}

	// TODO: Explore HostKeyCallback options for better security
	config := &ssh.ClientConfig{
		User: cfg.User,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // will update later
		Timeout:         5 * time.Second,
	}

	log.Printf("Connecting to %s@%s...\n", cfg.User[0:4], add[0:3])
	client, err := ssh.Dial("tcp", add+":22", config)
	if err != nil {
		log.Printf("failed to dial: %v", err)
		return nil, apperror.ErrInternal

	}

	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		log.Printf("failed to create session: %v", err)
		return nil, apperror.ErrInternal

	}
	defer session.Close()

	cmd := fmt.Sprintf("tail -n %d %s", lineCount, cfg.LogPath)
	outputBytes, err := session.CombinedOutput(cmd)
	if err != nil {
		log.Printf("failed to run command: %v\nOutput: %s", err, string(outputBytes))
		return nil, apperror.ErrInternal

	}

	rawOutput := string(outputBytes)
	res := parseAndCleanLogs(rawOutput)

	return res, nil
}

func parseAndCleanLogs(rawOutput string) *[]models.LogItem {
	var entries []models.LogItem
	lines := strings.SplitSeq(rawOutput, "\n")

	for line := range lines {
		line = strings.TrimSpace(line)

		matches := logRegex.FindStringSubmatch(line)

		// If no match, it means it's a stack trace line or empty.
		// We Skip it effectively "hiding" the stack trace.
		if matches == nil {
			continue
		}

		timestamp := matches[1]
		level := matches[2]
		src := matches[3]
		message := matches[4]

		if strings.Contains(message, RCON_STRING) {
			continue
		}

		if strings.Contains(level, "/") {
			parts := strings.Split(level, "/")
			if len(parts) > 1 {
				level = parts[len(parts)-1]
			}
		}

		srcParts := strings.Split(src, ".")
		src = strings.ReplaceAll(srcParts[len(srcParts)-1], "/", "")

		// at this point we are ready to redact.
		message = redactMessage(message)

		if len(message) > MAX_MSG_LENGTH {
			message = message[:MAX_MSG_LENGTH] + "..."
		}

		entries = append(entries, models.LogItem{
			Timestamp: timestamp,
			Level:     level,
			Message:   message,
			Src:       src,
		})
	}

	return &entries
}

// helper that applies all regexes to try to redact potentially sensitive info
func redactMessage(msg string) string {
	for _, rule := range redactionRules {
		msg = rule.Pattern.ReplaceAllString(msg, rule.Replacement)
	}
	return msg
}
