package app

import (
	"fmt"
	"log/slog"
	"time"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
	"gopkg.in/gomail.v2"
)

type MailerFrom struct {
	Name    string `mapstructure:"name"`
	Address string `mapstructure:"address"`
}

type MailerServer struct {
	Address  string `mapstructure:"address"`
	Port     int    `mapstructure:"port"`
	UserName string `mapstructure:"user_name"`
	Password string `mapstructure:"password"`
}

type Mailer struct {
	Preview bool         `mapstructure:"preview"`
	From    MailerFrom   `mapstructure:"from"`
	Bcc     string       `mapstructure:"bcc"`
	Server  MailerServer `mapstructure:"server"`

	app *App
}

func (m *Mailer) Logger() *slog.Logger {
	return m.app.Logger()
}

func New(a *App) (*Mailer, error) {
	m := Mailer{app: a}

	return &m, nil
}

func (m *Mailer) Send(addresses []string, subject, md, html string) error {
	for i := range addresses {
		addresses[i] = normalizeString(addresses[i])
	}

	m.Logger().Info("Sending mail", "to", addresses, "subject", subject)

	if m.Preview {
		fmt.Println(md)
		return nil
	}

	msg := gomail.NewMessage()
	msg.SetHeader("From", normalizeString(m.From.Name)+" <"+m.From.Address+">")

	msg.SetHeader("To", addresses...)
	msg.SetHeader("Cc", m.From.Address)
	msg.SetHeader("Subject", subject)

	if m.Bcc != "" {
		msg.SetHeader("Bcc", m.Bcc)
	}

	msg.SetBody("text/plain", md)
	msg.AddAlternative("text/html", html)

	d := gomail.NewDialer(m.Server.Address, m.Server.Port, m.Server.UserName, m.Server.Password)

	for {
		err := d.DialAndSend(msg)
		if err == nil {
			break
		}

		m.Logger().Error("Error sending mail", "to", addresses, "error", err)
		m.Logger().Info("Waiting for 10 seconds.")
		time.Sleep(10 * time.Second)
	}

	m.Logger().Info("Mail sent", "to", addresses)

	return nil
}

func normalizeString(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)

	n, _, err := transform.String(t, s)
	if err != nil {
		panic(err)
	}

	return n
}
