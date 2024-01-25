package main

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/sethvargo/go-envconfig"
)

// Options describes common configuration
type Options struct {
	APIAddress        string `env:"API_ADDRESS, required"`
	ClientID          string `env:"CLIENT_ID, required"`
	ClientSecret      string `env:"CLIENT_SECRET, required"`
	OrgPrefix         string `env:"ORG_PREFIX, required"`
	NotifyDays        int    `env:"NOTIFY_DAYS, default=25"`
	PurgeDays         int    `env:"PURGE_DAYS, default=30"`
	MailSender        string `env:"MAIL_SENDER, required"`
	NotifyMailSubject string `env:"NOTIFY_MAIL_SUBJECT, required"`
	PurgeMailSubject  string `env:"PURGE_MAIL_SUBJECT, required"`
	DryRun            bool   `env:"DRY_RUN, default=true"`
	TimeStartsAt      string `env:"TIME_STARTS_AT"`
	SMTPOptions
}

func main() {
	var opts Options
	ctx := context.Background()

	if err := envconfig.Process(ctx, &opts); err != nil {
		log.Fatalf("error parsing options: %s", err.Error())
	}

	cfClient, err := newCFClient(
		opts.APIAddress,
		opts.ClientID,
		opts.ClientSecret,
	)
	if err != nil {
		log.Fatalf("error creating client: %s", err.Error())
	}

	orgs, err := listSandboxOrgs(ctx, cfClient, opts.OrgPrefix)
	if err != nil {
		log.Fatalf("error getting orgs: %s", err.Error())
	}

	// Build filter of users with email addresses (not service accounts)
	users, err := cfClient.Users.ListAll(ctx, nil)
	if err != nil {
		log.Fatalf("error getting users: %s", err.Error())
	}
	userGUIDs := map[string]bool{}
	for _, user := range users {
		if strings.Contains(user.Username, "@") {
			userGUIDs[user.GUID] = true
		}
	}

	now := time.Now().Truncate(24 * time.Hour)

	var timeStartsAt time.Time
	if opts.TimeStartsAt != "" {
		timeStartsAt, err = time.Parse(time.RFC3339Nano, opts.TimeStartsAt)
		if err != nil {
			log.Fatalf("error parsing time starts at: %s", err.Error())
		}
	}

	var allPurgeErrors []string

	for _, org := range orgs {
		log.Printf("getting org resources for org %s", org.Name)
		spaces, apps, instances, err := listOrgResources(ctx, cfClient, org)
		if err != nil {
			log.Fatalf("error listing org resources for org %s: %s", org.Name, err.Error())
		}

		toNotify, toPurge, err := listPurgeSpaces(spaces, apps, instances, now, opts.NotifyDays, opts.PurgeDays, timeStartsAt)
		if err != nil {
			log.Fatalf("error listing spaces to purge for org %s: %s", org.Name, err.Error())
		}

		smtpMailer := &SMTPMailer{
			options: opts.SMTPOptions,
		}

		log.Printf("notifying %d spaces in org %s", len(toNotify), org.Name)
		for _, details := range toNotify {
			err = notifySpaceUsers(ctx, cfClient, opts, userGUIDs, org, details, smtpMailer)
			if err != nil {
				log.Fatalf("error notifying space %s in org %s: %s", details.Space.Name, org.Name, err)
			}
		}

		log.Printf("purging %d spaces in org %s", len(toPurge), org.Name)
		for _, details := range toPurge {
			err = purgeAndRecreateSpace(ctx, cfClient, opts, userGUIDs, org, details, smtpMailer)
			if err != nil {
				allPurgeErrors = append(allPurgeErrors, err.Error())
			}
		}
	}

	if len(allPurgeErrors) > 0 {
		log.Fatalf("error(s) purging sandboxes: %s", strings.Join(allPurgeErrors, ", "))
	}
}
