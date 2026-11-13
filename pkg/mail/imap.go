package mail

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	stdmail "net/mail"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
	imapclient "github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-message"
	"github.com/emersion/go-message/charset"
	msgmail "github.com/emersion/go-message/mail"
	"github.com/gomarkdown/markdown"
	"github.com/jovandeginste/spark-personal-assistant/pkg/router"
	gomail "github.com/wneessen/go-mail"
)

type IMAPConfig struct {
	Host string
	Port int
}

type SMTPConfig struct {
	Host string
	Port int
}

type Config struct {
	IMAP      IMAPConfig
	SMTP      SMTPConfig
	To        string
	Allowlist []string
	Username  string
	Password  string
	Folder    string
	UseTLS    bool
}

type Message struct {
	Subject     string
	From        string
	FromAddress string
	Date        time.Time
	Body        string
}

const (
	pollInterval   = time.Minute
	maxMessageSize = 10 * 1024 * 1024
)

type idleResult struct {
	command *imapclient.IdleCommand
	err     error
}

func ConnectAndIdle(ctx context.Context, cfg Config, msgChan chan<- Message, logger *slog.Logger) error {
	addr := fmt.Sprintf("%s:%d", cfg.IMAP.Host, cfg.IMAP.Port)

	updates := make(chan struct{}, 1)

	options := &imapclient.Options{
		TLSConfig: &tls.Config{
			ServerName: cfg.IMAP.Host,
			MinVersion: tls.VersionTLS12,
		},
		WordDecoder: &mime.WordDecoder{CharsetReader: charset.Reader},
		UnilateralDataHandler: &imapclient.UnilateralDataHandler{
			Mailbox: func(*imapclient.UnilateralDataMailbox) {
				select {
				case updates <- struct{}{}:
				default:
				}
			},
		},
	}

	var (
		c   *imapclient.Client
		err error
	)

	if cfg.UseTLS {
		c, err = imapclient.DialTLS(addr, options)
	} else {
		c, err = imapclient.DialInsecure(addr, options)
	}

	if err != nil {
		return fmt.Errorf("failed to connect to imap server: %w", err)
	}
	defer func() { _ = c.Close() }()

	if err := c.Login(cfg.Username, cfg.Password).Wait(); err != nil {
		return fmt.Errorf("failed to login: %w", err)
	}

	folder := cfg.Folder
	if folder == "" {
		folder = "INBOX"
	}

	if _, err := c.Select(folder, nil).Wait(); err != nil {
		return fmt.Errorf("failed to select folder %s: %w", folder, err)
	}

	messages, err := fetchUnseen(c, cfg, logger)
	if err != nil {
		logger.Error("failed to fetch initial unseen messages", "error", err)
	} else {
		for _, m := range messages {
			msgChan <- m
		}
	}

	if !c.Caps().Has(imap.CapIdle) {
		return pollUnseen(ctx, c, cfg, logger, msgChan)
	}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		idleCh := make(chan idleResult, 1)

		go func() {
			command, err := c.Idle()
			idleCh <- idleResult{command: command, err: err}
		}()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case result := <-idleCh:
			if result.err != nil {
				return fmt.Errorf("idle failed: %w", result.err)
			}

			select {
			case <-updates:
				if err := result.command.Close(); err != nil {
					return fmt.Errorf("idle disconnected: %w", err)
				}

				newMessages, err := fetchUnseen(c, cfg, logger)
				if err != nil {
					logger.Error("failed to fetch unseen messages", "error", err)
					continue
				}

				for _, m := range newMessages {
					msgChan <- m
				}
			case <-ctx.Done():
				_ = result.command.Close()
				return ctx.Err()
			}
		}
	}
}

func pollUnseen(ctx context.Context, c *imapclient.Client, cfg Config, logger *slog.Logger, msgChan chan<- Message) error {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			messages, err := fetchUnseen(c, cfg, logger)
			if err != nil {
				logger.Error("failed to fetch unseen messages", "error", err)
				continue
			}

			for _, m := range messages {
				msgChan <- m
			}
		}
	}
}

func fetchUnseen(c *imapclient.Client, cfg Config, logger *slog.Logger) ([]Message, error) {
	criteria := &imap.SearchCriteria{NotFlag: []imap.Flag{imap.FlagSeen}}

	data, err := c.UIDSearch(criteria, nil).Wait()
	if err != nil {
		return nil, fmt.Errorf("failed to search messages: %w", err)
	}

	uids := data.AllUIDs()
	logger.Info("IMAP search found unseen messages", "count", len(uids))
	if len(uids) == 0 {
		return []Message{}, nil
	}

	uidSet := imap.UIDSetNum(uids...)

	metadata, err := c.Fetch(uidSet, &imap.FetchOptions{
		Envelope:   true,
		RFC822Size: true,
	}).Collect()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch message metadata: %w", err)
	}

	type candidate struct {
		uid imap.UID
		msg Message
	}

	candidates := make([]candidate, 0, len(metadata))
	bodyUIDs := make([]imap.UID, 0, len(metadata))

	for _, buffer := range metadata {
		if buffer.Envelope == nil {
			continue
		}

		m := Message{
			Subject: buffer.Envelope.Subject,
			Date:    buffer.Envelope.Date,
		}

		if len(buffer.Envelope.From) > 0 {
			m.From = buffer.Envelope.From[0].Name
			m.FromAddress = buffer.Envelope.From[0].Addr()
			if m.From == "" {
				m.From = m.FromAddress
			}
		}

		if !isAllowedSender(cfg, m.FromAddress) {
			logger.Info("Ignoring email from non-allowlisted sender", "from", m.FromAddress)

			continue
		}

		if buffer.RFC822Size > maxMessageSize {
			logger.Warn("Skipping oversized email", "subject", m.Subject, "size", buffer.RFC822Size)

			continue
		}

		candidates = append(candidates, candidate{uid: buffer.UID, msg: m})
		bodyUIDs = append(bodyUIDs, buffer.UID)
	}

	if len(bodyUIDs) > 0 {
		bodies, err := c.Fetch(imap.UIDSetNum(bodyUIDs...), &imap.FetchOptions{
			BodySection: []*imap.FetchItemBodySection{{Peek: true}},
		}).Collect()
		if err != nil {
			return nil, fmt.Errorf("failed to fetch message bodies: %w", err)
		}

		bodyByUID := make(map[imap.UID][]byte, len(bodies))
		for _, buffer := range bodies {
			if len(buffer.BodySection) > 0 {
				bodyByUID[buffer.UID] = buffer.BodySection[0].Bytes
			}
		}

		results := make([]Message, 0, len(candidates))
		for _, cand := range candidates {
			if body, ok := bodyByUID[cand.uid]; ok {
				text, err := extractMessageBody(bytes.NewReader(body))
				if err != nil {
					logger.Error("failed to decode message body", "error", err)

					continue
				}

				cand.msg.Body = text
			}

			results = append(results, cand.msg)
		}

		markSeen(c, uidSet, logger)

		return results, nil
	}

	markSeen(c, uidSet, logger)

	return []Message{}, nil
}

func markSeen(c *imapclient.Client, uidSet imap.UIDSet, logger *slog.Logger) {
	if err := c.Store(uidSet, &imap.StoreFlags{
		Op:     imap.StoreFlagsAdd,
		Silent: true,
		Flags:  []imap.Flag{imap.FlagSeen},
	}, nil).Close(); err != nil {
		logger.Error("failed to mark messages as seen", "error", err)
	}
}

// extractMessageBody decodes transfer encodings, charsets and multipart
// bodies, preferring the text/plain part over HTML.
func extractMessageBody(r io.Reader) (string, error) {
	reader, err := msgmail.CreateReader(r)
	if err != nil {
		if reader == nil || (!message.IsUnknownCharset(err) && !message.IsUnknownEncoding(err)) {
			return "", err
		}
	}

	var plainBody, htmlBody string

	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			if message.IsUnknownCharset(err) || message.IsUnknownEncoding(err) {
				continue
			}

			return "", err
		}

		mediaType := "text/plain"
		if contentType := part.Header.Get("Content-Type"); contentType != "" {
			parsedType, _, err := mime.ParseMediaType(contentType)
			if err != nil {
				continue
			}

			mediaType = parsedType
		}

		switch {
		case plainBody == "" && strings.HasPrefix(mediaType, "text/plain"):
			body, err := io.ReadAll(part.Body)
			if err != nil {
				return "", err
			}

			plainBody = string(body)
		case htmlBody == "" && strings.HasPrefix(mediaType, "text/html"):
			body, err := io.ReadAll(part.Body)
			if err != nil {
				return "", err
			}

			htmlBody = string(body)
		}
	}

	if plainBody != "" {
		return plainBody, nil
	}

	return htmlBody, nil
}

func isAllowedSender(cfg Config, address string) bool {
	if len(cfg.Allowlist) == 0 {
		return true
	}

	address, ok := parseEmailAddress(address)
	if !ok {
		return false
	}

	for _, allowed := range cfg.Allowlist {
		allowedAddress, ok := parseEmailAddress(allowed)
		if ok && strings.EqualFold(address, allowedAddress) {
			return true
		}
	}

	return false
}

// Register registers the Mail instance as an address to the central router with full SMTP sending capability.
func Register(r *router.Router, cfg Config, instanceName string, description string, logger *slog.Logger) error {
	if instanceName == "" {
		instanceName = "default"
	}
	addrID := instanceName + "@mail"
	return r.RegisterAddress(router.Address{
		ID:           addrID,
		InstanceName: instanceName,
		System:       "mail",
		Description:  description,
		SendFunc: func(ctx context.Context, msg router.Message) error {
			return sendEmail(cfg, msg, logger)
		},
	})
}

func sendEmail(cfg Config, msg router.Message, logger *slog.Logger) error {
	if logger != nil {
		logger.Info("Sending email via SMTP/IMAP account", "to", msg.Metadata.To, "subject", msg.Metadata.Subject)
	}

	to := extractRecipients(cfg, msg)
	if len(to) == 0 {
		return errors.New("no valid recipient email address found in 'to' or 'from_address'")
	}

	smtpHost := cfg.SMTP.Host
	if smtpHost == "" {
		smtpHost = cfg.IMAP.Host
	}
	smtpPort := cfg.SMTP.Port
	if smtpPort == 0 {
		smtpPort = 587
		if cfg.IMAP.Port == 993 || cfg.IMAP.Port == 465 {
			smtpPort = 465
		}
	}
	mailMsg := gomail.NewMsg()

	if err := mailMsg.From(cfg.Username); err != nil {
		return fmt.Errorf("invalid sender %q: %w", cfg.Username, err)
	}

	if err := mailMsg.To(to...); err != nil {
		return fmt.Errorf("invalid recipient: %w", err)
	}

	mailMsg.Subject(sanitizeHeaderValue(msg.Metadata.Subject))
	mailMsg.SetBodyString(gomail.TypeTextPlain, msg.Content)
	mailMsg.AddAlternativeString(gomail.TypeTextHTML, string(markdown.ToHTML([]byte(msg.Content), nil, nil)))

	opts := []gomail.Option{
		gomail.WithPort(smtpPort),
	}

	if cfg.Username != "" && cfg.Password != "" {
		opts = append(opts,
			gomail.WithSMTPAuth(gomail.SMTPAuthPlain),
			gomail.WithUsername(cfg.Username),
			gomail.WithPassword(cfg.Password),
		)
	}

	if smtpPort == 465 {
		opts = append(opts, gomail.WithSSL())
	} else {
		opts = append(opts, gomail.WithTLSPolicy(gomail.TLSOpportunistic))
	}

	smtpClient, err := gomail.NewClient(smtpHost, opts...)
	if err != nil {
		return fmt.Errorf("failed to create smtp client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := smtpClient.DialAndSendWithContext(ctx, mailMsg); err != nil {
		if logger != nil {
			logger.Error("Failed to send email", "error", err)
		}

		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

func extractRecipients(cfg Config, msg router.Message) []string {
	candidateRecipients := collectCandidateRecipients(msg)
	allowed := filterByAllowlist(candidateRecipients, cfg.Allowlist)

	if len(allowed) > 0 {
		return allowed
	}

	// Fallback to default cfg.To if specified
	if cfg.To != "" {
		return []string{cfg.To}
	}

	return nil
}

func sanitizeHeaderValue(value string) string {
	replacer := strings.NewReplacer("\r\n", " ", "\r", " ", "\n", " ")

	return strings.TrimSpace(replacer.Replace(value))
}

func collectCandidateRecipients(msg router.Message) []string {
	var recipients []string
	recipients = appendValidRecipient(recipients, msg.OriginalSource)

	for _, to := range msg.Metadata.To {
		recipients = appendValidRecipient(recipients, to)
	}

	if len(recipients) == 0 && msg.Metadata.Extra != nil {
		if extraTo, ok := msg.Metadata.Extra["to"].(string); ok {
			recipients = appendValidRecipient(recipients, extraTo)
		}
		if len(recipients) == 0 {
			if fromAddr, ok := msg.Metadata.Extra["from_address"].(string); ok {
				recipients = appendValidRecipient(recipients, fromAddr)
			}
		}
	}

	return recipients
}

func appendValidRecipient(recipients []string, raw string) []string {
	address, ok := parseEmailAddress(raw)
	if !ok || isRouterAddress(address) {
		return recipients
	}

	for _, existing := range recipients {
		if strings.EqualFold(existing, address) {
			return recipients
		}
	}

	return append(recipients, address)
}

func parseEmailAddress(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}

	parsed, err := stdmail.ParseAddress(raw)
	if err != nil {
		return "", false
	}

	return parsed.Address, true
}

func isRouterAddress(value string) bool {
	v := strings.ToLower(strings.TrimSpace(value))
	return strings.HasSuffix(v, "@mail") || strings.HasSuffix(v, "@matrix") || strings.HasSuffix(v, "@web")
}

func filterByAllowlist(candidates, allowlist []string) []string {
	if len(allowlist) == 0 {
		return candidates
	}

	allowSet := make(map[string]struct{}, len(allowlist))
	for _, allowedAddr := range allowlist {
		address, ok := parseEmailAddress(allowedAddr)
		if !ok {
			continue
		}
		allowSet[strings.ToLower(address)] = struct{}{}
	}

	filtered := make([]string, 0, len(candidates))
	for _, cand := range candidates {
		if _, ok := allowSet[strings.ToLower(cand)]; ok {
			filtered = append(filtered, cand)
		}
	}

	return filtered
}
