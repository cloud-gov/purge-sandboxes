package main

import (
	"html/template"
	"log"

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
	PurgeCreateDays   int    `envconfig:"purge_create_days" default:"30"`
	PurgeActivityDays int    `envconfig:"purge_activity_days" default:"5"`
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

	for _, org := range orgs {
		spaces, apps, instances, err := sandbox.ListOrgResources(client, org)
		if err != nil {
			log.Fatalf("error listing org resources for org %s: %s", org.Name, err.Error())
		}

		// Notify owners of spaces to be purged
		toNotify, err := sandbox.ListNotifySpaces(spaces, apps, instances, opts.NotifyDays)
		if err != nil {
			log.Fatalf("error getting spaces to purge for org %s: %s", org.Name, err.Error())
		}
		for _, space := range toNotify {
			recipients, err := sandbox.ListRecipients(space)
			if err != nil {
				log.Fatalf("error listing recipients on space %s: %s", space.Name, err.Error())
			}
			if err := sandbox.SendMail(opts.SMTPOptions, notifyTemplate, space, recipients); err != nil {
				log.Fatalf("error sending mail on space %s: %s", space.Name, err.Error())
			}
		}

		// Purge spaces
		toPurge, err := sandbox.ListPurgeSpaces(spaces, apps, instances, opts.PurgeCreateDays, opts.PurgeActivityDays)
		if err != nil {
			log.Fatalf("error getting spaces to purge for org %s: %s", org.Name, err.Error())
		}
		for _, space := range toPurge {
			recipients, err := sandbox.ListRecipients(space)
			if err != nil {
				log.Fatalf("error listing recipients on space %s: %s", space.Name, err.Error())
			}
			if err := sandbox.SendMail(opts.SMTPOptions, purgeTemplate, space, recipients); err != nil {
				log.Fatalf("error sending mail on space %s: %s", space.Name, err.Error())
			}
			if err := client.DeleteSpace(space.Guid, true, true); err != nil {
				log.Fatalf("error deleting space %s: %s", space.Name, err.Error())
			}
		}
	}
}
