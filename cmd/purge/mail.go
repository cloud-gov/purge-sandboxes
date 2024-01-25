package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"html/template"

	"gopkg.in/gomail.v2"
)

// SMTPOptions describes configation for sending mail via SMTP
type SMTPOptions struct {
	SMTPHost string `env:"SMTP_HOST, required"`
	SMTPPort int    `env:"SMTP_PORT, default=587"`
	SMTPUser string `env:"SMTP_USER, required"`
	SMTPPass string `env:"SMTP_PASS, required"`
	SMTPCert string `env:"SMTP_CERT"`
}

type mailer interface {
	sendMail(
		opts SMTPOptions,
		sender string,
		subject string,
		body string,
		recipients []string,
	) error
}

type smtpMailer struct {
	options SMTPOptions
}

// renderTemplate renders a template to string
func renderTemplate(tmpl *template.Template, data map[string]interface{}) (string, error) {
	buf := bytes.Buffer{}
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// sendMail sends email via SMTP
func (m *smtpMailer) sendMail(
	opts SMTPOptions,
	sender string,
	subject string,
	body string,
	recipients []string,
) error {
	if len(recipients) == 0 {
		return nil
	}

	d := gomail.NewDialer(opts.SMTPHost, opts.SMTPPort, opts.SMTPUser, opts.SMTPPass)
	if opts.SMTPCert != "" {
		pool := x509.NewCertPool()
		pool.AppendCertsFromPEM([]byte(opts.SMTPCert))
		d.TLSConfig = &tls.Config{
			ServerName: opts.SMTPHost,
			RootCAs:    pool,
		}
	}
	s, err := d.Dial()
	if err != nil {
		return err
	}

	msg := gomail.NewMessage()
	msg.SetHeaders(map[string][]string{
		"From":    {sender},
		"Subject": {subject},
		"To":      recipients,
	})
	msg.SetBody("text/html", body)
	return gomail.Send(s, msg)
}
