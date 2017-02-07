package main

import (
	"bytes"
	"fmt"
	"log"
	"net/url"
	"text/template"

	"github.com/cloudfoundry-community/go-cfclient"
	"github.com/kelseyhightower/envconfig"
	"gopkg.in/gomail.v2"
)

type Options struct {
	APIAddress   string   `envconfig:"api_address" required:"true"`
	ClientID     string   `envconfig:"client_id" required:"true"`
	ClientSecret string   `envconfig:"client_secret" required:"true"`
	OrgName      string   `envconfig:"org_name" required:"true"`
	ServiceNames []string `envconfig:"service_names" required:"true"`
	SMTPHost     string   `envconfig:"smtp_host" required:"true"`
	SMTPPort     int      `envconfig:"smtp_port" default:"587"`
	SMTPUser     string   `envconfig:"smtp_user" required:"true"`
	SMTPPass     string   `envconfig:"smtp_pass" required:"true"`
	MailSender   string   `envconfig:"mail_sender" required:"true"`
	MailSubject  string   `envconfig:"mail_subject" required:"true"`
}

func getServiceByName(client *cfclient.Client, service string) (cfclient.Service, error) {
	q := url.Values{}
	q.Set("q", fmt.Sprintf("label:%s", service))
	services, err := client.ListServicesByQuery(q)
	if err != nil {
		return cfclient.Service{}, err
	}
	if len(services) != 1 {
		return cfclient.Service{}, fmt.Errorf("could not find service %s", service)
	}
	return services[0], nil
}

// listInstances gets a map from space guids to lists of relevant service instances
func listInstances(client *cfclient.Client, orgGUID string, services map[string]bool) (map[string][]cfclient.ServiceInstance, error) {
	q := url.Values{}
	q.Set("q", fmt.Sprintf("organization_guid:%s", orgGUID))
	instances, err := client.ListServiceInstancesByQuery(q)
	m := map[string][]cfclient.ServiceInstance{}
	if err != nil {
		return m, err
	}
	for _, instance := range instances {
		if _, ok := services[instance.ServiceGuid]; ok {
			if _, ok := m[instance.SpaceGuid]; !ok {
				m[instance.SpaceGuid] = []cfclient.ServiceInstance{}
			}
			m[instance.SpaceGuid] = append(m[instance.SpaceGuid], instance)
		}
	}
	return m, nil
}

// listSpaces gets a map from space guids to spaces
func listSpaces(client *cfclient.Client, orgGUID string) (map[string]cfclient.Space, error) {
	q := url.Values{}
	q.Set("q", fmt.Sprintf("organization_guid:%s", orgGUID))
	spaces, err := client.ListSpacesByQuery(q)
	if err != nil {
		return nil, err
	}
	m := make(map[string]cfclient.Space, len(spaces))
	for _, space := range spaces {
		m[space.Guid] = space
	}
	return m, nil
}

func sendMail(
	opts Options,
	tmpl *template.Template,
	instances []cfclient.ServiceInstance,
	space cfclient.Space,
	recipient string,
) error {
	b := bytes.Buffer{}
	if err := tmpl.Execute(&b, map[string]interface{}{
		"space":     space,
		"instances": instances,
	}); err != nil {
		return err
	}

	log.Printf("sending to %s: %s", recipient, b.String())

	d := gomail.NewDialer(opts.SMTPHost, opts.SMTPPort, opts.SMTPUser, opts.SMTPPass)
	s, err := d.Dial()
	if err != nil {
		return err
	}

	m := gomail.NewMessage()
	m.SetHeader("From", opts.MailSender)
	m.SetHeader("Subject", opts.MailSubject)
	m.SetAddressHeader("To", recipient, recipient)
	m.SetBody("text/plain", b.String())
	return gomail.Send(s, m)
}

func main() {
	var opts Options
	if err := envconfig.Process("", &opts); err != nil {
		log.Fatalf("error parsing options: %s", err.Error())
	}

	tmpl, err := template.ParseFiles("./email.tmpl")
	if err != nil {
		log.Fatalf("error reading template: %s", err.Error())
	}

	client, err := cfclient.NewClient(&cfclient.Config{
		ApiAddress:   opts.APIAddress,
		ClientID:     opts.ClientID,
		ClientSecret: opts.ClientSecret,
	})
	if err != nil {
		log.Fatalf("error creating client: %s", err.Error())
	}

	org, err := client.GetOrgByName(opts.OrgName)
	if err != nil {
		log.Fatalf("error getting org: %s", err.Error())
	}

	services := make(map[string]bool, len(opts.ServiceNames))
	for _, label := range opts.ServiceNames {
		service, err := getServiceByName(client, label)
		if err != nil {
			log.Fatal(err.Error())
		}
		services[service.Guid] = true
	}

	instances, err := listInstances(client, org.Guid, services)
	if err != nil {
		log.Fatalf("error listing instances: %s", err.Error())
	}

	spaces, err := listSpaces(client, org.Guid)
	if err != nil {
		log.Fatalf("error listing spaces: %s", err.Error())
	}

	for spaceGuid, instances := range instances {
		if space, ok := spaces[spaceGuid]; ok {
			recipient := fmt.Sprintf("%s@%s", space.Name, org.Name)
			if err := sendMail(opts, tmpl, instances, space, recipient); err != nil {
				log.Fatalf("error sending mail", err.Error())
			}
		}
	}
}
