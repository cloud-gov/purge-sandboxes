package main

import (
	"context"
	"fmt"
	"html/template"
	"log"

	"github.com/cloudfoundry-community/go-cfclient/v3/client"
	"github.com/cloudfoundry-community/go-cfclient/v3/resource"
)

func purgeAndRecreateSpace(
	ctx context.Context,
	cfClient *cfResourceClient,
	opts Options,
	userGUIDs map[string]bool,
	org *resource.Organization,
	details SpaceDetails,
	mailSender mailer,
) error {
	roleListOpts := client.NewRoleListOptions()
	roleListOpts.SpaceGUIDs.Values = []string{details.Space.GUID}
	spaceRoles, spaceUsers, err := cfClient.Roles.ListIncludeUsersAll(ctx, roleListOpts)
	if err != nil {
		return fmt.Errorf("error listing roles with users on space %s: %w", details.Space.Name, err)
	}

	recipients, err := listRecipients(userGUIDs, spaceUsers)
	if err != nil {
		return fmt.Errorf("error listing recipients on space %s: %w", details.Space.Name, err)
	}

	developers, managers := listSpaceDevsAndManagers(userGUIDs, spaceRoles, spaceUsers)
	log.Printf("Purging space %s; recipients: %+v, developers: %+v, managers: %+v", details.Space.Name, recipients, developers, managers)

	if opts.DryRun {
		return nil
	}

	if err := sendPurgeEmail(opts, org, details, recipients, mailSender); err != nil {
		return fmt.Errorf("error sending purge notification email for space %s in org %s: %w", details.Space.Name, org.Name, err)
	}

	log.Printf("deleting and recreating space %s", details.Space.Name)
	if err := purgeSpace(ctx, cfClient, details.Space); err != nil {
		return fmt.Errorf("error purging space %s in org %s: %w", details.Space.Name, org.Name, err)
	}

	if len(developers) > 0 || len(managers) > 0 {
		spaceRequest := &resource.SpaceCreate{
			Name:          details.Space.Name,
			Relationships: details.Space.Relationships,
		}
		if _, err := cfClient.Spaces.Create(ctx, spaceRequest); err != nil {
			return fmt.Errorf("error recreating space %s in org %s: %w", details.Space.Name, org.Name, err)
		}
		log.Printf("recreating space roles")
		if err := recreateSpaceDevsAndManagers(ctx, cfClient, details.Space.GUID, developers, managers); err != nil {
			return fmt.Errorf("error recreating space developers/managers for space %s in org %s: %w", details.Space.Name, org.Name, err)
		}
	}

	return nil
}

func sendPurgeEmail(
	opts Options,
	org *resource.Organization,
	details SpaceDetails,
	recipients []string,
	mailSender mailer,
) error {
	purgeTemplate, err := template.ParseFiles("../../templates/base.html", "../../templates/purge.tmpl")
	if err != nil {
		return fmt.Errorf("error reading purge template: %s", err)
	}

	data := map[string]interface{}{
		"org":   org,
		"space": details.Space,
		"days":  opts.PurgeDays,
	}
	body, err := renderTemplate(purgeTemplate, data)
	if err != nil {
		return fmt.Errorf("error rendering email: %s", err)
	}

	log.Printf("sending to %s: %s", recipients, body)
	if err := mailSender.sendMail(opts.SMTPOptions, opts.MailSender, opts.PurgeMailSubject, body, recipients); err != nil {
		return fmt.Errorf("error sending mail on space %s: %w", details.Space.Name, err)
	}

	return nil
}
