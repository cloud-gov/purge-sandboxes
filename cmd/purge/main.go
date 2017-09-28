package main

import (
	"html/template"
	"log"
	"time"

	"github.com/cloudfoundry-community/go-cfclient"
	"github.com/kelseyhightower/envconfig"

	"github.com/18F/cg-sandbox/sandbox"
)

type Options struct {
	APIAddress        string `envconfig:"api_address" required:"true"`
	ClientID          string `envconfig:"client_id" required:"true"`
	ClientSecret      string `envconfig:"client_secret" required:"true"`
	OrgPrefix         string `envconfig:"org_prefix" required:"true"`
	NotifyDays        int    `envconfig:"notify_days" default:"25"`
	PurgeDays         int    `envconfig:"purge_create_days" default:"30"`
	MailSender        string `envconfig:"mail_sender" required:"true"`
	NotifyMailSubject string `envconfig:"notify_mail_subject" required:"true"`
	PurgeMailSubject  string `envconfig:"purge_mail_subject" required:"true"`
	DryRun            bool   `envconfig:"dry_run" default:"true"`
	sandbox.SMTPOptions
}

func main() {
	var opts Options
	if err := envconfig.Process("", &opts); err != nil {
		log.Fatalf("error parsing options: %s", err.Error())
	}

	notifyTemplate, err := template.ParseFiles("./notify.tmpl")
	if err != nil {
		log.Fatalf("error reading notify template: %s", err.Error())
	}

	purgeTemplate, err := template.ParseFiles("./purge.tmpl")
	if err != nil {
		log.Fatalf("error reading purge template: %s", err.Error())
	}

	client, err := cfclient.NewClient(&cfclient.Config{
		ApiAddress:   opts.APIAddress,
		ClientID:     opts.ClientID,
		ClientSecret: opts.ClientSecret,
	})
	if err != nil {
		log.Fatalf("error creating client: %s", err.Error())
	}

	orgs, err := sandbox.ListSandboxOrgs(client, opts.OrgPrefix)
	if err != nil {
		log.Fatalf("error getting orgs: %s", err.Error())
	}

	now := time.Now().Truncate(24 * time.Hour)

	for _, org := range orgs {
		spaces, apps, instances, err := sandbox.ListOrgResources(client, org)
		if err != nil {
			log.Fatalf("error listing org resources for org %s: %s", org.Name, err.Error())
		}

		toNotify, toPurge, err := sandbox.ListPurgeSpaces(spaces, apps, instances, now, opts.NotifyDays, opts.PurgeDays)
		if err != nil {
			log.Fatalf("error listing spaces to purge for org %s: %s", org.Name, err.Error())
		}

		for _, space := range toNotify {
			recipients, err := sandbox.ListRecipients(space)
			if err != nil {
				log.Fatalf("error listing recipients on space %s: %s", space.Name, err.Error())
			}
			log.Printf("Notifying space %s; recipients %+v", space.Name, recipients)
			if !opts.DryRun {
				data := map[string]interface{}{
					"space": space,
					"days":  opts.PurgeDays - opts.NotifyDays,
				}
				if err := sandbox.SendMail(opts.SMTPOptions, opts.MailSender, opts.NotifyMailSubject, notifyTemplate, data, recipients); err != nil {
					log.Fatalf("error sending mail on space %s: %s", space.Name, err.Error())
				}
			}
		}

		for _, space := range toPurge {
			recipients, err := sandbox.ListRecipients(space)
			if err != nil {
				log.Fatalf("error listing recipients on space %s: %s", space.Name, err.Error())
			}
			log.Printf("Purging space %s; recipients %+v", space.Name, recipients)
			if !opts.DryRun {
				data := map[string]interface{}{"space": space}
				if err := sandbox.SendMail(opts.SMTPOptions, opts.MailSender, opts.PurgeMailSubject, purgeTemplate, data, recipients); err != nil {
					log.Fatalf("error sending mail on space %s: %s", space.Name, err.Error())
				}
				if err := client.DeleteSpace(space.Guid, true, true); err != nil {
					log.Fatalf("error deleting space %s: %s", space.Name, err.Error())
				}
			}
		}
	}
}
