package mail

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	stdmail "net/mail"
	"net/smtp"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	idle "github.com/emersion/go-imap-idle"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/charset"
	"github.com/gomarkdown/markdown"
	"github.com/jovandeginste/spark-personal-assistant/pkg/router"
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

func init() {
	imap.CharsetReader = charset.Reader
}

func ConnectAndIdle(cfg Config, msgChan chan<- Message, logger *slog.Logger) error {
	addr := fmt.Sprintf("%s:%d", cfg.IMAP.Host, cfg.IMAP.Port)
	var c *client.Client
	var err error

	if cfg.UseTLS {
		c, err = client.DialTLS(addr, &tls.Config{
			ServerName: cfg.IMAP.Host,
			MinVersion: tls.VersionTLS12,
		})
	} else {
		c, err = client.Dial(addr)
	}

	if err != nil {
		return fmt.Errorf("failed to connect to imap server: %w", err)
	}
	defer c.Logout()

	if err := c.Login(cfg.Username, cfg.Password); err != nil {
		return fmt.Errorf("failed to login: %w", err)
	}

	folder := cfg.Folder
	if folder == "" {
		folder = "INBOX"
	}

	_, err = c.Select(folder, false)
	if err != nil {
		return fmt.Errorf("failed to select folder %s: %w", folder, err)
	}

	// Fetch any existing unseen messages first
	msgs, err := fetchUnseen(c, logger)
	if err != nil {
		logger.Error("failed to fetch initial unseen messages", "error", err)
	} else {
		for _, m := range msgs {
			msgChan <- m
		}
	}

	idleClient := idle.NewClient(c)

	updates := make(chan client.Update, 50)
	c.Updates = updates

	for {
		stop := make(chan struct{})
		done := make(chan error, 1)

		go func() {
			done <- idleClient.IdleWithFallback(stop, 0)
		}()

		select {
		case update := <-updates:
			if err := handleUpdate(update, c, logger, stop, done, msgChan); err != nil {
				return err
			}
		case err := <-done:
			if err != nil {
				return fmt.Errorf("idle disconnected: %w", err)
			}
		}
	}
}

func handleUpdate(update client.Update, c *client.Client, logger *slog.Logger, stop chan struct{}, done chan error, msgChan chan<- Message) error {
	logger.Info("IMAP update received", "type", fmt.Sprintf("%T", update))

	close(stop)
	err := <-done // Wait for IDLE to finish
	if err != nil {
		return fmt.Errorf("idle disconnected: %w", err)
	}

	newMsgs, err := fetchUnseen(c, logger)
	if err != nil {
		logger.Error("failed to fetch unseen messages", "error", err)
		return nil
	}

	if len(newMsgs) == 0 {
		return nil
	}

	logger.Info("Fetched new unseen messages", "count", len(newMsgs))
	for _, m := range newMsgs {
		msgChan <- m
	}
	logger.Info("IMAP updates parsed")
	return nil
}

func fetchUnseen(c *client.Client, logger *slog.Logger) ([]Message, error) {
	criteria := imap.NewSearchCriteria()
	criteria.WithoutFlags = []string{imap.SeenFlag}

	// Ensure we don't hang if server doesn't respond
	c.Timeout = 10 * time.Second
	defer func() { c.Timeout = 0 }()

	uids, err := c.Search(criteria)
	if err != nil {
		return nil, fmt.Errorf("failed to search messages: %w", err)
	}
	logger.Info("IMAP search found unseen messages", "count", len(uids))
	if len(uids) == 0 {
		return []Message{}, nil
	}

	seqset := new(imap.SeqSet)
	seqset.AddNum(uids...)

	section := &imap.BodySectionName{}
	items := []imap.FetchItem{imap.FetchEnvelope, section.FetchItem()}

	messages := make(chan *imap.Message, 10)
	done := make(chan error, 1)

	go func() {
		done <- c.Fetch(seqset, items, messages)
	}()

	results := make([]Message, 0, len(uids))
	for msg := range messages {
		if msg == nil {
			continue
		}

		m := Message{
			Subject: msg.Envelope.Subject,
			Date:    msg.Envelope.Date,
		}

		if len(msg.Envelope.From) > 0 {
			m.From = msg.Envelope.From[0].PersonalName
			m.FromAddress = msg.Envelope.From[0].MailboxName + "@" + msg.Envelope.From[0].HostName
			if m.From == "" {
				m.From = m.FromAddress
			}
		}

		r := msg.GetBody(section)
		if r != nil {
			parsed, err := stdmail.ReadMessage(r)
			if err != nil {
				logger.Error("failed to read message body", "error", err)
				continue
			}

			body, err := io.ReadAll(parsed.Body)
			if err != nil {
				logger.Error("failed to read message body", "error", err)
				continue
			}
			m.Body = string(body)
		}

		results = append(results, m)
	}

	if err := <-done; err != nil {
		return nil, fmt.Errorf("failed to fetch messages: %w", err)
	}

	// Mark as seen
	flags := []any{imap.SeenFlag}
	item := imap.FormatFlagsOp(imap.AddFlags, true)
	if err := c.Store(seqset, item, flags, nil); err != nil {
		logger.Error("failed to mark messages as seen", "error", err)
	}

	return results, nil
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
	auth := smtp.Auth(nil)
	if cfg.Username != "" && cfg.Password != "" {
		auth = smtp.PlainAuth("", cfg.Username, cfg.Password, smtpHost)
	}

	msgBytes := buildMultipartMessage(cfg.Username, to, msg.Metadata.Subject, msg.Content)
	smtpAddr := fmt.Sprintf("%s:%d", smtpHost, smtpPort)

	var err error
	if smtpPort == 465 {
		err = sendSMTPSecure(smtpAddr, smtpHost, auth, cfg.Username, to, msgBytes)
	} else {
		err = smtp.SendMail(smtpAddr, auth, cfg.Username, to, msgBytes)
	}

	if err != nil {
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

func buildMultipartMessage(from string, to []string, subject, plainText string) []byte {
	if subject == "" {
		subject = "Response from Spark Assistant"
	}

	htmlContent := markdown.ToHTML([]byte(plainText), nil, nil)
	boundary := "----=_Part_Spark_Assistant_Boundary_123456789"

	var msgBuilder strings.Builder
	msgBuilder.WriteString(fmt.Sprintf("From: %s\r\n", from))
	msgBuilder.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(to, ", ")))
	msgBuilder.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	msgBuilder.WriteString("MIME-Version: 1.0\r\n")
	msgBuilder.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=\"%s\"\r\n\r\n", boundary))

	msgBuilder.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	msgBuilder.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	msgBuilder.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	msgBuilder.WriteString(plainText)
	msgBuilder.WriteString("\r\n\r\n")

	msgBuilder.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	msgBuilder.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	msgBuilder.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	msgBuilder.Write(htmlContent)
	msgBuilder.WriteString("\r\n\r\n")

	msgBuilder.WriteString(fmt.Sprintf("--%s--\r\n", boundary))

	return []byte(msgBuilder.String())
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

func sendSMTPSecure(smtpAddr, smtpHost string, auth smtp.Auth, username string, to []string, msgBytes []byte) error {
	tlsconfig := &tls.Config{
		ServerName: smtpHost,
		MinVersion: tls.VersionTLS12,
	}
	conn, err := tls.Dial("tcp", smtpAddr, tlsconfig)
	if err != nil {
		return fmt.Errorf("failed to TLS dial smtp: %w", err)
	}
	defer conn.Close()

	smtpClient, err := smtp.NewClient(conn, smtpHost)
	if err != nil {
		return fmt.Errorf("failed to create smtp client: %w", err)
	}
	defer smtpClient.Quit()

	if auth != nil {
		if err := smtpClient.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth failed: %w", err)
		}
	}
	if err = smtpClient.Mail(username); err != nil {
		return err
	}
	for _, recipient := range to {
		if err = smtpClient.Rcpt(recipient); err != nil {
			return err
		}
	}
	w, err := smtpClient.Data()
	if err != nil {
		return err
	}
	_, err = w.Write(msgBytes)
	if err != nil {
		return err
	}
	return w.Close()
}
