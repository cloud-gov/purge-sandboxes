package main

import (
	"fmt"
	"html/template"
	"log"
	"strings"
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
	TimeStartsAt      string `envconfig:"time_starts_at"`
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

	var timeStartsAt time.Time
	if opts.TimeStartsAt != "" {
		timeStartsAt, err = time.Parse(time.RFC3339Nano, opts.TimeStartsAt)
		if err != nil {
			log.Fatalf("error parsing time starts at: %s", err.Error())
		}
	}

	var purgeErrors []string

	for _, org := range orgs {
		spaces, apps, instances, err := sandbox.ListOrgResources(client, org)
		if err != nil {
			log.Fatalf("error listing org resources for org %s: %s", org.Name, err.Error())
		}

		toNotify, toPurge, err := sandbox.ListPurgeSpaces(spaces, apps, instances, now, opts.NotifyDays, opts.PurgeDays, timeStartsAt)
		if err != nil {
			log.Fatalf("error listing spaces to purge for org %s: %s", org.Name, err.Error())
		}

		log.Printf("notifying %d spaces in org %s", len(toNotify), org.Name)
		for _, space := range toNotify {
			recipients, _, _, err := sandbox.ListRecipients(space)
			if err != nil {
				log.Fatalf("error listing recipients on space %s: %s", space.Name, err.Error())
			}
			log.Printf("Notifying space %s; recipients %+v", space.Name, recipients)
			if !opts.DryRun {
				data := map[string]interface{}{
					"space": space,
					"days":  opts.PurgeDays - opts.NotifyDays,
				}
				body, err := sandbox.RenderTemplate(notifyTemplate, data)
				log.Printf("sending to %s: %s", recipients, body)
				if err != nil {
					log.Fatalf("error rendering email: %s", err.Error())
				}
				if err := sandbox.SendMail(opts.SMTPOptions, opts.MailSender, opts.NotifyMailSubject, body, recipients); err != nil {
					log.Fatalf("error sending mail on space %s: %s", space.Name, err.Error())
				}
			}
		}

		log.Printf("purging %d spaces in org %s", len(toPurge), org.Name)
		for _, space := range toPurge {
			recipients, developers, managers, err := sandbox.ListRecipients(space)
			if err != nil {
				log.Fatalf("error listing recipients on space %s: %s", space.Name, err.Error())
			}
			log.Printf("Purging space %s; recipients %+v", space.Name, recipients)
			if !opts.DryRun {
				data := map[string]interface{}{"space": space}
				body, err := sandbox.RenderTemplate(purgeTemplate, data)
				log.Printf("sending to %s: %s", recipients, body)
				if err != nil {
					log.Fatalf("error rendering email: %s", err.Error())
				}
				if err := sandbox.SendMail(opts.SMTPOptions, opts.MailSender, opts.PurgeMailSubject, body, recipients); err != nil {
					log.Fatalf("error sending mail on space %s: %s", space.Name, err.Error())
				}
				log.Printf("deleting and recreating space %s", space.Name)
				if err := client.DeleteSpace(space.Guid, true, false); err != nil {
					purgeErrors = append(purgeErrors, fmt.Sprintf("error purging space %s in org %s: %s", space.Name, org.Name, err.Error()))
				} else {
					if len(developers) > 0 || len(managers) > 0 {
						spaceRequest := cfclient.SpaceRequest{
							Name:              space.Name,
							OrganizationGuid:  space.OrganizationGuid,
							SpaceQuotaDefGuid: space.QuotaDefinitionGuid,
							DeveloperGuid:     developers,
							ManagerGuid:       managers,
						}
						log.Printf("recreating space: %+v", spaceRequest)
						if _, err := client.CreateSpace(spaceRequest); err != nil {
							log.Fatalf("error recreating space %s: %s", space.Name, err.Error())
						}
					}
				}
			}
		}
	}

	if len(purgeErrors) > 0 {
		log.Fatalf("error(s) purging sandboxes: %s", strings.Join(purgeErrors, ", "))
	}
}
