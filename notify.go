package main

import (
	"fmt"
	"log"
	"net/url"

	"github.com/cloudfoundry-community/go-cfclient"
	"github.com/kelseyhightower/envconfig"
)

type Options struct {
	APIAddress   string   `envconfig:"api_address" required:"true"`
	ClientID     string   `envconfig:"client_id" required:"true"`
	ClientSecret string   `envconfig:"client_secret" required:"true"`
	OrgName      string   `envconfig:"org_name" required:"true"`
	ServiceGUIDs []string `envconfig:"service_guids" required:"true"`
}

func listInstances(client *cfclient.Client, orgGUID string) ([]cfclient.ServiceInstance, error) {
	q := url.Values{}
	q.Set("q", fmt.Sprintf("organization_guid:%s", orgGUID))
	return client.ListServiceInstancesByQuery(q)
}

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

func sendMail(instance cfclient.ServiceInstance, space cfclient.Space) error {
	return nil
}

func main() {
	var opts Options
	if err := envconfig.Process("", &opts); err != nil {
		log.Fatalf("error parsing options: %s", err.Error())
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

	instances, err := listInstances(client, org.Guid)
	if err != nil {
		log.Fatalf("error listing instances: %s", err.Error())
	}

	spaces, err := listSpaces(client, org.Guid)

	services := make(map[string]bool, len(opts.ServiceGUIDs))
	for _, service := range opts.ServiceGUIDs {
		services[service] = true
	}

	for _, instance := range instances {
		if _, ok := services[instance.ServicePlanGuid]; ok {
			sendMail(instance, spaces[instance.SpaceGuid])
		}
	}
}
