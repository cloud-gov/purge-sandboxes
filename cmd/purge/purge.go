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
) error {
	purgeTemplate, err := template.ParseFiles("./templates/base.html", "./templates/purge.tmpl")
	if err != nil {
		log.Fatalf("error reading purge template: %s", err.Error())
	}

	spaceRoles, err := cfClient.Roles.ListAll(ctx, &client.RoleListOptions{
		SpaceGUIDs: client.Filter{
			Values: []string{details.Space.GUID},
		},
		Include: resource.RoleIncludeUser,
	})
	if err != nil {
		log.Fatalf("error listing roles on space %s: %s", details.Space.Name, err.Error())
	}

	spaceUsers, err := cfClient.Spaces.ListUsersAll(ctx, details.Space.GUID, nil)
	if err != nil {
		return fmt.Errorf("error listing users on space %s: %w", details.Space.Name, err)
	}

	recipients, err := ListRecipients(userGUIDs, spaceUsers)
	if err != nil {
		return fmt.Errorf("error listing recipients on space %s: %w", details.Space.Name, err)
	}

	developers, managers := ListSpaceDevsAndManagers(userGUIDs, spaceRoles)
	log.Printf("Purging space %s; recipients: %+v, developers: %+v, managers: %+v", details.Space.Name, recipients, developers, managers)

	if !opts.DryRun {
		data := map[string]interface{}{
			"org":   org,
			"space": details.Space,
			"days":  opts.PurgeDays,
		}
		body, err := RenderTemplate(purgeTemplate, data)
		if err != nil {
			log.Fatalf("error rendering email: %s", err.Error())
		}

		log.Printf("sending to %s: %s", recipients, body)
		if err := SendMail(opts.SMTPOptions, opts.MailSender, opts.PurgeMailSubject, body, recipients); err != nil {
			log.Fatalf("error sending mail on space %s: %s", details.Space.Name, err.Error())
		}

		log.Printf("deleting and recreating space %s", details.Space.Name)
		if err := PurgeSpace(ctx, cfClient, details.Space); err != nil {
			return fmt.Errorf("error purging space %s in org %s: %w", details.Space.Name, org.Name, err)
		}

		if len(developers) > 0 || len(managers) > 0 {
			spaceRequest := &resource.SpaceCreate{
				Name:          details.Space.Name,
				Relationships: details.Space.Relationships,
			}
			log.Printf("recreating space: %+v", spaceRequest)
			if _, err := cfClient.Spaces.Create(ctx, &resource.SpaceCreate{}); err != nil {
				return fmt.Errorf("error recreating space %s in org %s: %w", details.Space.Name, org.Name, err)
			}
			log.Printf("recreating space roles")
			if err := RecreateSpaceDevsAndManagers(ctx, cfClient, details.Space.GUID, developers, managers); err != nil {
				return fmt.Errorf("error recreating space developers/managers for space %s in org %s: %w", details.Space.Name, org.Name, err)
			}
		}
	}

	return nil
}
