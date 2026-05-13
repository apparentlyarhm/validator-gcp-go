package util

import (
	"context"
	"fmt"
	"log"
	"net"
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
func FetchLogs(ctx context.Context, cfg *config.SSHConfig, lineCount int, add string) (*[]models.LogItem, error) {
	// IMPORTANT: read this

	// under load, the vm can actually fail in a half-state where it accepts TCP
	// but does nothing afterwards. we started getting this problem in the cobbleverse server
	// powered by docker.

	// the ssh library doesnt respect context. so, Timeout in ClientConfig only applies to the initial TCP
	// AFTER its connected and has performed the handshake, the goroutine at the end will handle timeout there.
	// that leaves us with the handshake phase, which WILL hang if not handled by a custom dialer.

	// we now pass the ctx from upstream with timeout to hard cancel the command if the vm is
	// in this vegetative state.

	key, e := config.GetPrivateKey(cfg.PKeyB64)
	if e != nil {
		return nil, apperror.ErrInternal
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, apperror.ErrInternal
	}
	eh := "string" // testing

	// TODO: Explore HostKeyCallback options for better security
	config := &ssh.ClientConfig{
		User: cfg.User,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			// Calculate fingerprint
			fingerprint := ssh.FingerprintSHA256(key)

			// Compare
			if fingerprint != eh {
				return fmt.Errorf("host key mismatch: got %s, expected %s", fingerprint, eh)
			}
			log.Printf("Host key verified: %s\n", fingerprint)
			return nil
		},
		Timeout: 5 * time.Second,
	}

	log.Printf("Connecting to %s@%s...\n", cfg.User[0:4], add[0:3])

	// based on the above, this is our custom dialer.
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", add+":22")
	if err != nil {
		log.Printf("failed to dial tcp: %v", err)
		return nil, apperror.ErrInternal
	}

	deadline, ok := ctx.Deadline()
	if ok {
		_ = conn.SetDeadline(deadline)
	}

	// perform the SSH handshake over the TCP connection
	c, chans, reqs, err := ssh.NewClientConn(conn, add+":22", config)
	if err != nil {
		_ = conn.Close()
		log.Printf("possible vm vegetative state: %v", err)
		return nil, apperror.ErrInternal
	}
	// we have to remove the deadline in case the actual command takes time
	_ = conn.SetDeadline(time.Time{})

	// create client
	client := ssh.NewClient(c, chans, reqs)
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		log.Printf("failed to create session: %v", err)
		return nil, apperror.ErrInternal

	}
	defer session.Close()

	cmd := fmt.Sprintf("tail -n %d %s", lineCount, cfg.LogPath)

	// this bit spawns a goroutine to run the command and waits for either the command to finish or the context to timeout/cancel.
	done := make(chan struct{}) // empty channel
	var outputBytes []byte
	var execErr error

	go func() {
		outputBytes, execErr = session.CombinedOutput(cmd)
		close(done) // close the open channel to signal completion
	}()

	select {
	case <-done:
		if execErr != nil {
			log.Printf("failed to run command: %v\nOutput: %s", execErr, string(outputBytes))
			return nil, apperror.ErrInternal
		}

	case <-ctx.Done():
		_ = session.Close()
		_ = client.Close()
		return nil, ctx.Err()
	}

	rawOutput := string(outputBytes)
	res := parseAndCleanLogs(rawOutput)

	return res, nil
}

func parseAndCleanLogs(rawOutput string) *[]models.LogItem {
	var entries []models.LogItem
	var rawFallback []models.LogItem

	lines := strings.SplitSeq(rawOutput, "\n")
	validMatchesFound := 0

	for line := range lines {
		line = strings.TrimSpace(line)

		safeRawMsg := redactMessage(line)
		if len(safeRawMsg) > MAX_MSG_LENGTH {
			safeRawMsg = safeRawMsg[:MAX_MSG_LENGTH] + "..."
		}

		rawFallback = append(rawFallback, models.LogItem{
			Timestamp: "UNKNOWN",
			Level:     "RAW",
			Src:       "IDK",
			Message:   safeRawMsg,
		})

		matches := logRegex.FindStringSubmatch(line)

		// If no match, it means it's a stack trace line or empty.
		// We Skip it effectively "hiding" the stack trace.
		if matches == nil {
			continue
		}

		validMatchesFound++

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

	// if we parsed lines, but NOTHING matched the regex,
	// the regex is likely broken. Return the raw lines instead.
	if validMatchesFound == 0 && len(rawFallback) > 0 {
		log.Println("WARNING: Log format changed! Zero regex matches. Falling back to RAW output.")
		return &rawFallback
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
