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
	SMTPHost string `envconfig:"smtp_host" required:"true"`
	SMTPPort int    `envconfig:"smtp_port" default:"587"`
	SMTPUser string `envconfig:"smtp_user" required:"true"`
	SMTPPass string `envconfig:"smtp_pass" required:"true"`
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
	sender string,
	subject string,
	tmpl *template.Template,
	data map[string]interface{},
	recipients []string,
) error {
	b := bytes.Buffer{}
	if err := tmpl.Execute(&b, data); err != nil {
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
		"From":    {sender},
		"Subject": {subject},
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

	return sandboxes, nil
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

func GetFirstResource(
	space cfclient.Space,
	apps []cfclient.App,
	instances []cfclient.ServiceInstance,
) (time.Time, error) {
	groupedApps := groupAppsBySpace(apps)
	groupedInstances := groupInstancesBySpace(instances)

	var firstResource time.Time
	for _, app := range groupedApps[space.Guid] {
		createdAt, err := time.Parse(time.RFC3339Nano, app.CreatedAt)
		if err != nil {
			return firstResource, err
		}
		if firstResource.IsZero() || createdAt.Before(firstResource) {
			firstResource = createdAt
		}
	}
	for _, instance := range groupedInstances[space.Guid] {
		createdAt, err := time.Parse(time.RFC3339Nano, instance.CreatedAt)
		if err != nil {
			return firstResource, err
		}
		if firstResource.IsZero() || createdAt.Before(firstResource) {
			firstResource = createdAt
		}
	}

	return firstResource, nil
}

func ListPurgeSpaces(
	spaces []cfclient.Space,
	apps []cfclient.App,
	instances []cfclient.ServiceInstance,
	now time.Time,
	notifyThreshold int,
	purgeThreshold int,
) (
	toNotify []cfclient.Space,
	toPurge []cfclient.Space,
	err error,
) {
	var firstResource time.Time
	for _, space := range spaces {
		firstResource, err = GetFirstResource(space, apps, instances)
		if err != nil {
			return
		}
		if firstResource.IsZero() {
			continue
		}

		delta := int(now.Sub(firstResource.Truncate(24*time.Hour)).Hours() / 24)
		if delta == notifyThreshold {
			toNotify = append(toNotify, space)
		} else if delta >= purgeThreshold {
			toPurge = append(toPurge, space)
		}
	}
	return
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
