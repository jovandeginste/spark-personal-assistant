package imap

import (
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net/mail"
	"time"

	"github.com/emersion/go-imap"
	idle "github.com/emersion/go-imap-idle"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/charset"
)

type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	Folder   string
	UseTLS   bool
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
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	var c *client.Client
	var err error

	if cfg.UseTLS {
		c, err = client.DialTLS(addr, &tls.Config{
			ServerName: cfg.Host,
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
			parsed, err := mail.ReadMessage(r)
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
