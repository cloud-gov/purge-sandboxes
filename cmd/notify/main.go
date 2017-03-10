package main

import (
	"bytes"
	"fmt"
	"log"
	"net/mail"
	"net/url"
	"text/template"

	"github.com/cloudfoundry-community/go-cfclient"
	"github.com/kelseyhightower/envconfig"
	"gopkg.in/gomail.v2"
)

type Options struct {
	APIAddress   string `envconfig:"api_address" required:"true"`
	ClientID     string `envconfig:"client_id" required:"true"`
	ClientSecret string `envconfig:"client_secret" required:"true"`
	OrgName      string `envconfig:"org_name" required:"true"`
	SMTPHost     string `envconfig:"smtp_host" required:"true"`
	SMTPPort     int    `envconfig:"smtp_port" default:"587"`
	SMTPUser     string `envconfig:"smtp_user" required:"true"`
	SMTPPass     string `envconfig:"smtp_pass" required:"true"`
	MailSender   string `envconfig:"mail_sender" required:"true"`
	MailSubject  string `envconfig:"mail_subject" required:"true"`
}

// listSpaces gets a map from space guids to spaces
func listSpaces(client *cfclient.Client, orgGUID string) ([]cfclient.Space, error) {
	q := url.Values{}
	q.Set("q", fmt.Sprintf("organization_guid:%s", orgGUID))
	return client.ListSpacesByQuery(q)
}

// listRecipients get a list of recipient emails from a space
func listRecipients(space cfclient.Space) ([]string, error) {
	recipients := []string{}
	roles, err := space.Roles()
	if err != nil {
		return recipients, err
	}
	for _, role := range roles {
		if _, err := mail.ParseAddress(role.Username); err == nil {
			recipients = append(recipients, role.Username)
		}
	}
	return recipients, nil
}

func sendMail(
	opts Options,
	tmpl *template.Template,
	space cfclient.Space,
	recipients []string,
) error {
	b := bytes.Buffer{}
	if err := tmpl.Execute(&b, map[string]interface{}{
		"space": space,
	}); err != nil {
		return err
	}

	log.Printf("sending to %s: %s", recipients, b.String())

	d := gomail.NewDialer(opts.SMTPHost, opts.SMTPPort, opts.SMTPUser, opts.SMTPPass)
	s, err := d.Dial()
	if err != nil {
		return err
	}

	m := gomail.NewMessage()
	m.SetHeaders(map[string][]string{
		"From":    {opts.MailSender},
		"Subject": {opts.MailSubject},
		"To":      recipients,
	})
	m.SetBody("text/plain", b.String())
	return gomail.Send(s, m)
}

func main() {
	var opts Options
	if err := envconfig.Process("", &opts); err != nil {
		log.Fatalf("error parsing options: %s", err.Error())
	}

	tmpl, err := template.ParseFiles("./email.tmpl")
	if err != nil {
		log.Fatalf("error reading template: %s", err.Error())
	}

	client, err := cfclient.NewClient(&cfclient.Config{
		ApiAddress:   opts.APIAddress,
		ClientID:     opts.ClientID,
		ClientSecret: opts.ClientSecret,
	})
	if err != nil {
		log.Fatalf("error creating client: %s", err.Error())
	}

	org, err := client.GetOrgByName(opts.OrgName)
	if err != nil {
		log.Fatalf("error getting org: %s", err.Error())
	}

	spaces, err := listSpaces(client, org.Guid)
	if err != nil {
		log.Fatalf("error listing spaces: %s", err.Error())
	}

	for _, space := range spaces {
		recipients, err := listRecipients(space)
		if err != nil {
			log.Fatalf("error listing recipients", err.Error())
		}
		if err := sendMail(opts, tmpl, space, recipients); err != nil {
			log.Fatalf("error sending mail", err.Error())
		}
	}
}
