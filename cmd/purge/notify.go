package main

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"time"

	"github.com/cloudfoundry-community/go-cfclient/v3/resource"
)

func notifySpaceUsers(
	ctx context.Context,
	cfClient *cfResourceClient,
	opts Options,
	userGUIDs map[string]bool,
	org *resource.Organization,
	details SpaceDetails,
) error {
	notifyTemplate, err := template.ParseFiles("../../templates/base.html", "../../templates/notify.tmpl")
	if err != nil {
		return fmt.Errorf("error reading notify template: %w", err)
	}

	spaceUsers, err := cfClient.Spaces.ListUsersAll(ctx, details.Space.GUID, nil)
	if err != nil {
		return fmt.Errorf("error listing users on space %s: %w", details.Space.Name, err)
	}

	recipients, err := listRecipients(userGUIDs, spaceUsers)
	if err != nil {
		return fmt.Errorf("error listing recipients on space %s: %w", details.Space.Name, err)
	}

	log.Printf("Notifying space %s; recipients %+v", details.Space.Name, recipients)
	if !opts.DryRun {
		data := map[string]interface{}{
			"org":   org,
			"space": details.Space,
			"date":  details.Timestamp.Add(24 * time.Duration(opts.PurgeDays) * time.Hour),
			"days":  opts.PurgeDays,
		}

		body, err := renderTemplate(notifyTemplate, data)
		if err != nil {
			return fmt.Errorf("error rendering email: %w", err)
		}

		log.Printf("sending to %s: %s", recipients, body)

		if err := sendMail(opts.SMTPOptions, opts.MailSender, opts.NotifyMailSubject, body, recipients); err != nil {
			return fmt.Errorf("error sending mail on space %s: %w", details.Space.Name, err)
		}
	}

	return nil
}
