package main

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"strings"
	"time"

	"github.com/cloudfoundry-community/go-cfclient/v3/client"
	"github.com/cloudfoundry-community/go-cfclient/v3/config"
	"github.com/cloudfoundry-community/go-cfclient/v3/resource"

	"github.com/sethvargo/go-envconfig"

	"github.com/18f/cg-sandbox/sandbox"
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
	sandbox.SMTPOptions
}

func main() {
	var opts Options
	ctx := context.Background()

	if err := envconfig.Process(ctx, &opts); err != nil {
		log.Fatalf("error parsing options: %s", err.Error())
	}

	notifyTemplate, err := template.ParseFiles("./templates/base.html", "./templates/notify.tmpl")
	if err != nil {
		log.Fatalf("error reading notify template: %s", err.Error())
	}

	purgeTemplate, err := template.ParseFiles("./templates/base.html", "./templates/purge.tmpl")
	if err != nil {
		log.Fatalf("error reading purge template: %s", err.Error())
	}

	cfg, err := config.NewClientSecret(
		opts.APIAddress,
		opts.ClientID,
		opts.ClientSecret,
	)
	if err != nil {
		log.Fatalf("error creating client: %s", err.Error())
	}
	cfClient, err := client.New(cfg)
	if err != nil {
		log.Fatalf("error creating client: %s", err.Error())
	}

	orgs, err := sandbox.ListSandboxOrgs(ctx, cfClient, opts.OrgPrefix)
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

	var purgeErrors []string

	for _, org := range orgs {
		spaces, apps, instances, err := sandbox.ListOrgResources(ctx, cfClient, org)
		if err != nil {
			log.Fatalf("error listing org resources for org %s: %s", org.Name, err.Error())
		}

		toNotify, toPurge, err := sandbox.ListPurgeSpaces(spaces, apps, instances, now, opts.NotifyDays, opts.PurgeDays, timeStartsAt)
		if err != nil {
			log.Fatalf("error listing spaces to purge for org %s: %s", org.Name, err.Error())
		}

		var (
			spaceUsers []*resource.User
			spaceRoles []*resource.Role
			recipients []string
		)

		log.Printf("notifying %d spaces in org %s", len(toNotify), org.Name)
		for _, details := range toNotify {
			spaceUsers, err = cfClient.Spaces.ListUsersAll(ctx, details.Space.GUID, nil)
			if err != nil {
				log.Fatalf("error listing roles on space %s: %s", details.Space.Name, err.Error())
			}

			recipients, err = sandbox.ListRecipients(userGUIDs, spaceUsers)
			if err != nil {
				log.Fatalf("error listing recipients on space %s: %s", details.Space.Name, err.Error())
			}

			log.Printf("Notifying space %s; recipients %+v", details.Space.Name, recipients)
			if !opts.DryRun {
				data := map[string]interface{}{
					"org":   org,
					"space": details.Space,
					"date":  details.Timestamp.Add(24 * time.Duration(opts.PurgeDays) * time.Hour),
					"days":  opts.PurgeDays,
				}
				body, err := sandbox.RenderTemplate(notifyTemplate, data)
				log.Printf("sending to %s: %s", recipients, body)
				if err != nil {
					log.Fatalf("error rendering email: %s", err.Error())
				}
				if err := sandbox.SendMail(opts.SMTPOptions, opts.MailSender, opts.NotifyMailSubject, body, recipients); err != nil {
					log.Fatalf("error sending mail on space %s: %s", details.Space.Name, err.Error())
				}
			}
		}

		log.Printf("purging %d spaces in org %s", len(toPurge), org.Name)
		for _, details := range toPurge {
			spaceRoles, err = cfClient.Roles.ListAll(ctx, &client.RoleListOptions{
				SpaceGUIDs: client.Filter{
					Values: []string{details.Space.GUID},
				},
				Include: resource.RoleIncludeUser,
			})
			if err != nil {
				log.Fatalf("error listing roles on space %s: %s", details.Space.Name, err.Error())
			}

			developers, managers := sandbox.ListSpaceDevsAndManagers(userGUIDs, spaceRoles)
			log.Printf("Purging space %s; recipients %+v", details.Space.Name, recipients)
			if !opts.DryRun {
				data := map[string]interface{}{
					"org":   org,
					"space": details.Space,
					"days":  opts.PurgeDays,
				}
				body, err := sandbox.RenderTemplate(purgeTemplate, data)
				log.Printf("sending to %s: %s", recipients, body)
				if err != nil {
					log.Fatalf("error rendering email: %s", err.Error())
				}
				if err := sandbox.SendMail(opts.SMTPOptions, opts.MailSender, opts.PurgeMailSubject, body, recipients); err != nil {
					log.Fatalf("error sending mail on space %s: %s", details.Space.Name, err.Error())
				}
				log.Printf("deleting and recreating space %s", details.Space.Name)
				if err := sandbox.PurgeSpace(cfClient, details.Space); err != nil {
					purgeErrors = append(purgeErrors, fmt.Sprintf("error purging space %s in org %s: %s", details.Space.Name, org.Name, err.Error()))
					break
				}
				if len(developers) > 0 || len(managers) > 0 {
					spaceRequest := &resource.SpaceCreate{
						Name:          details.Space.Name,
						Relationships: details.Space.Relationships,
					}
					log.Printf("recreating space: %+v", spaceRequest)
					if _, err := cfClient.Spaces.Create(ctx, &resource.SpaceCreate{}); err != nil {
						purgeErrors = append(purgeErrors, fmt.Sprintf("error recreating space %s in org %s: %s", details.Space.Name, org.Name, err.Error()))
						break
					}
					log.Printf("recreating space roles")
					if err := sandbox.RecreateSpaceDevsAndManagers(ctx, cfClient, details.Space.GUID, developers, managers); err != nil {
						purgeErrors = append(purgeErrors, fmt.Sprintf("error recreating space developers/managers for space %s in org %s: %s", details.Space.Name, org.Name, err.Error()))
						break
					}
				}
			}
		}
	}

	if len(purgeErrors) > 0 {
		log.Fatalf("error(s) purging sandboxes: %s", strings.Join(purgeErrors, ", "))
	}
}
