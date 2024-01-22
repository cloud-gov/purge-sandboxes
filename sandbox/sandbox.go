package sandbox

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"html/template"
	"net/mail"
	"strings"
	"time"

	"github.com/cloudfoundry-community/go-cfclient/v3/client"
	"github.com/cloudfoundry-community/go-cfclient/v3/resource"
	"gopkg.in/gomail.v2"
)

// SMTPOptions describes configation for sending mail via SMTP
type SMTPOptions struct {
	SMTPHost string `envconfig:"smtp_host" required:"true"`
	SMTPPort int    `envconfig:"smtp_port" default:"587"`
	SMTPUser string `envconfig:"smtp_user" required:"true"`
	SMTPPass string `envconfig:"smtp_pass" required:"true"`
	SMTPCert string `envconfig:"smtp_cert"`
}

// ListRecipients get a list of recipient emails from space users
func ListRecipients(
	userGUIDs map[string]bool,
	spaceUsers []*resource.User,
) (addresses []string, err error) {
	addresses = []string{}
	for _, user := range spaceUsers {
		if _, ok := userGUIDs[user.GUID]; !ok {
			continue
		}

		if _, err := mail.ParseAddress(user.Username); err != nil {
			return nil, err
		}
		addresses = append(addresses, user.Username)
	}
	return addresses, nil
}

func ListSpaceDevsAndManagers(
	userGUIDs map[string]bool,
	spaceRoles []*resource.Role,
) (developers []string, managers []string) {
	developers = []string{}
	managers = []string{}
	for _, role := range spaceRoles {
		if _, ok := userGUIDs[role.Relationships.User.Data.GUID]; !ok {
			continue
		}

		if role.Type == resource.SpaceRoleDeveloper.String() {
			developers = append(developers, role.Relationships.User.Data.GUID)
		} else if role.Type == resource.SpaceRoleManager.String() {
			managers = append(managers, role.Relationships.User.Data.GUID)
		}
	}
	return
}

func RecreateSpaceDevsAndManagers(
	cfClient *client.Client,
	spaceGUID string,
	developers []string,
	managers []string,
) error {
	for _, developerGUID := range developers {
		_, err := cfClient.Roles.CreateSpaceRole(context.Background(), spaceGUID, developerGUID, resource.SpaceRoleDeveloper)
		if err != nil {
			return err
		}
	}
	for _, managerGUID := range managers {
		_, err := cfClient.Roles.CreateSpaceRole(context.Background(), spaceGUID, managerGUID, resource.SpaceRoleManager)
		if err != nil {
			return err
		}
	}
	return nil
}

// PurgeSpace deletes a space; if the delete fails, it deletes all applications within the space
func PurgeSpace(cfClient *client.Client, space *resource.Space) error {
	_, spaceErr := cfClient.Spaces.Delete(context.Background(), space.GUID)
	if spaceErr != nil {
		apps, err := cfClient.Applications.ListAll(context.Background(), &client.AppListOptions{
			SpaceGUIDs: client.Filter{
				Values: []string{space.GUID},
			},
		})
		if err != nil {
			return err
		}
		for _, app := range apps {
			_, err := cfClient.Applications.Delete(context.Background(), app.GUID)
			if err != nil {
				return err
			}
		}
		return spaceErr
	}
	return nil
}

// RenderTemplate renders a template to string
func RenderTemplate(tmpl *template.Template, data map[string]interface{}) (string, error) {
	buf := bytes.Buffer{}
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// SendMail sends email via SMTP
func SendMail(
	opts SMTPOptions,
	sender string,
	subject string,
	body string,
	recipients []string,
) error {
	if len(recipients) == 0 {
		return nil
	}

	d := gomail.NewDialer(opts.SMTPHost, opts.SMTPPort, opts.SMTPUser, opts.SMTPPass)
	if opts.SMTPCert != "" {
		pool := x509.NewCertPool()
		pool.AppendCertsFromPEM([]byte(opts.SMTPCert))
		d.TLSConfig = &tls.Config{
			ServerName: opts.SMTPHost,
			RootCAs:    pool,
		}
	}
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
	m.SetBody("text/html", body)
	return gomail.Send(s, m)
}

// ListSandboxOrgs lists all sandbox organizations
func ListSandboxOrgs(client *client.Client, prefix string) ([]*resource.Organization, error) {
	sandboxes := []*resource.Organization{}

	orgs, err := client.Organizations.ListAll(context.Background(), nil)
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

// ListOrgResources fetches apps, service instances, and spaces within an organization
func ListOrgResources(
	cfClient *client.Client,
	org *resource.Organization,
) (
	spaces []*resource.Space,
	apps []*resource.App,
	instances []*resource.ServiceInstance,
	err error,
) {
	// query := url.Values(map[string][]string{"q": []string{"organization_guid:" + org.Guid}})

	// apps, err = client.ListAppsByQuery(query)
	apps, err = cfClient.Applications.ListAll(context.Background(), &client.AppListOptions{
		OrganizationGUIDs: client.Filter{
			Values: []string{org.GUID},
		},
	})
	if err != nil {
		return
	}

	instances, err = cfClient.ServiceInstances.ListAll(context.Background(), &client.ServiceInstanceListOptions{
		OrganizationGUIDs: client.Filter{
			Values: []string{org.GUID},
		},
	})
	if err != nil {
		return
	}

	spaces, err = cfClient.Spaces.ListAll(context.Background(), &client.SpaceListOptions{
		OrganizationGUIDs: client.Filter{
			Values: []string{org.GUID},
		},
	})
	if err != nil {
		return
	}

	return
}

// GetFirstResource gets the creation timestamp of the earliest-created resource in a space
func GetFirstResource(
	space *resource.Space,
	apps []*resource.App,
	instances []*resource.ServiceInstance,
) (time.Time, error) {
	groupedApps := groupAppsBySpace(apps)
	groupedInstances := groupInstancesBySpace(instances)

	var firstResource time.Time
	for _, app := range groupedApps[space.GUID] {
		if firstResource.IsZero() || app.CreatedAt.Before(firstResource) {
			firstResource = app.CreatedAt
		}
	}
	for _, instance := range groupedInstances[space.GUID] {
		if firstResource.IsZero() || instance.CreatedAt.Before(firstResource) {
			firstResource = instance.CreatedAt
		}
	}

	return firstResource, nil
}

// SpaceDetails describes a space and its first resource creation time
type SpaceDetails struct {
	Timestamp time.Time
	Space     *resource.Space
}

// ListPurgeSpaces identifies spaces that will be notified or purged
func ListPurgeSpaces(
	spaces []*resource.Space,
	apps []*resource.App,
	instances []*resource.ServiceInstance,
	now time.Time,
	notifyThreshold int,
	purgeThreshold int,
	timeStartsAt time.Time,
) (
	toNotify []SpaceDetails,
	toPurge []SpaceDetails,
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
		if timeStartsAt.After(firstResource) {
			firstResource = timeStartsAt
		}

		firstResource := firstResource.Truncate(24 * time.Hour)
		delta := int(now.Sub(firstResource).Hours() / 24)
		if delta >= purgeThreshold {
			toPurge = append(toPurge, SpaceDetails{firstResource, space})
		} else if delta >= notifyThreshold {
			toNotify = append(toNotify, SpaceDetails{firstResource, space})
		}
	}
	return
}

func groupAppsBySpace(apps []*resource.App) map[string][]*resource.App {
	grouped := map[string][]*resource.App{}

	for _, app := range apps {
		spaceGuid := app.Relationships.Space.Data.GUID
		if _, ok := grouped[spaceGuid]; !ok {
			grouped[spaceGuid] = []*resource.App{}
		}
		grouped[spaceGuid] = append(grouped[spaceGuid], app)
	}

	return grouped
}

func groupInstancesBySpace(instances []*resource.ServiceInstance) map[string][]*resource.ServiceInstance {
	grouped := map[string][]*resource.ServiceInstance{}

	for _, instance := range instances {
		spaceGuid := instance.Relationships.Space.Data.GUID
		if _, ok := grouped[spaceGuid]; !ok {
			grouped[spaceGuid] = []*resource.ServiceInstance{}
		}
		grouped[spaceGuid] = append(grouped[spaceGuid], instance)
	}

	return grouped
}
