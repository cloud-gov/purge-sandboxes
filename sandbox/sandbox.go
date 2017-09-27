package sandbox

import (
	"bytes"
	"html/template"
	"log"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/cloudfoundry-community/go-cfclient"
	"gopkg.in/gomail.v2"
)

type SMTPOptions struct {
	SMTPHost    string `envconfig:"smtp_host" required:"true"`
	SMTPPort    int    `envconfig:"smtp_port" default:"587"`
	SMTPUser    string `envconfig:"smtp_user" required:"true"`
	SMTPPass    string `envconfig:"smtp_pass" required:"true"`
	MailSender  string `envconfig:"mail_sender" required:"true"`
	MailSubject string `envconfig:"mail_subject" required:"true"`
}

// ListRecipients get a list of recipient emails from a space
func ListRecipients(space cfclient.Space) ([]string, error) {
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

func SendMail(
	opts SMTPOptions,
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

func ListSandboxOrgs(client *cfclient.Client, prefix string) ([]cfclient.Org, error) {
	sandboxes := []cfclient.Org{}

	orgs, err := client.ListOrgs()
	if err != nil {
		return sandboxes, err
	}

	for _, org := range orgs {
		if strings.HasPrefix(org.Name, prefix) {
			sandboxes = append(sandboxes, org)
		}
	}

	return []cfclient.Org{}, nil
}

func groupAppsBySpace(apps []cfclient.App) map[string][]cfclient.App {
	grouped := map[string][]cfclient.App{}

	for _, app := range apps {
		if _, ok := grouped[app.SpaceGuid]; !ok {
			grouped[app.SpaceGuid] = []cfclient.App{}
		}
		grouped[app.SpaceGuid] = append(grouped[app.SpaceGuid], app)
	}

	return grouped
}

func groupInstancesBySpace(instances []cfclient.ServiceInstance) map[string][]cfclient.ServiceInstance {
	grouped := map[string][]cfclient.ServiceInstance{}

	for _, instance := range instances {
		if _, ok := grouped[instance.SpaceGuid]; !ok {
			grouped[instance.SpaceGuid] = []cfclient.ServiceInstance{}
		}
		grouped[instance.SpaceGuid] = append(grouped[instance.SpaceGuid], instance)
	}

	return grouped
}

func ListOrgResources(
	client *cfclient.Client,
	org cfclient.Org,
) (
	spaces []cfclient.Space,
	apps []cfclient.App,
	instances []cfclient.ServiceInstance,
	err error,
) {
	query := url.Values(map[string][]string{"q": []string{"organization_guid:" + org.Guid}})

	apps, err = client.ListAppsByQuery(query)
	if err != nil {
		return
	}

	instances, err = client.ListServiceInstancesByQuery(query)
	if err != nil {
		return
	}

	spaces, err = client.OrgSpaces(org.Guid)
	if err != nil {
		return
	}

	return
}

func ListNotifySpaces(
	spaces []cfclient.Space,
	apps []cfclient.App,
	instances []cfclient.ServiceInstance,
	threshold int,
) ([]cfclient.Space, error) {
	toNotify := []cfclient.Space{}
	now := time.Now()

	groupedApps := groupAppsBySpace(apps)
	groupedInstances := groupInstancesBySpace(instances)

	for _, space := range spaces {
		createdAt, err := time.Parse(time.RFC3339Nano, space.CreatedAt)
		if err != nil {
			return toNotify, err
		}

		if now.Sub(createdAt) > time.Duration(threshold*24)*time.Hour {
			if len(groupedApps[space.Guid]) > 0 || len(groupedInstances[space.Guid]) > 0 {
				toNotify = append(toNotify, space)
			}
		}
	}

	return toNotify, nil
}

func ListPurgeSpaces(
	spaces []cfclient.Space,
	apps []cfclient.App,
	instances []cfclient.ServiceInstance,
	spaceThreshold, activityThreshold int,
) ([]cfclient.Space, error) {
	toPurge := []cfclient.Space{}
	now := time.Now()

	groupedApps := groupAppsBySpace(apps)
	groupedInstances := groupInstancesBySpace(instances)

	for _, space := range spaces {
		createdAt, err := time.Parse(time.RFC3339Nano, space.CreatedAt)
		if err != nil {
			return toPurge, err
		}
		lastUpdated := createdAt
		if now.Sub(createdAt) > time.Duration(spaceThreshold*24)*time.Hour {
			for _, app := range groupedApps[space.Guid] {
				appCreatedAt, err := time.Parse(time.RFC3339Nano, app.CreatedAt)
				if err != nil {
					return toPurge, err
				}
				if appCreatedAt.Unix() > lastUpdated.Unix() {
					lastUpdated = appCreatedAt
				}
			}

			for _, instance := range groupedInstances[space.Guid] {
				instanceCreatedAt, err := time.Parse(time.RFC3339Nano, instance.CreatedAt)
				if err != nil {
					return toPurge, err
				}
				if instanceCreatedAt.Unix() > lastUpdated.Unix() {
					lastUpdated = instanceCreatedAt
				}
			}

			if now.Sub(lastUpdated) > time.Duration(activityThreshold*24)*time.Hour {
				toPurge = append(toPurge, space)
			}
		}
	}

	return toPurge, nil
}
