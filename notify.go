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
	ServiceNames []string `envconfig:"service_names" required:"true"`
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
	fmt.Println(fmt.Sprintf("%s %s", instance.Name, space.Name))
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
	if err != nil {
		log.Fatalf("error getting org: %s", err.Error())
	}

	instances, err := listInstances(client, org.Guid)
	if err != nil {
		log.Fatalf("error listing instances: %s", err.Error())
	}

	spaces, err := listSpaces(client, org.Guid)
	if err != nil {
		log.Fatalf("error listing spaces: %s", err.Error())
	}

	services := make(map[string]bool, len(opts.ServiceNames))
	for _, label := range opts.ServiceNames {
		service, err := getServiceByName(client, label)
		if err != nil {
			log.Fatal(err.Error())
		}
		services[service.Guid] = true
	}

	for _, instance := range instances {
		if _, ok := services[instance.ServiceGuid]; ok {
			sendMail(instance, spaces[instance.SpaceGuid])
		}
	}
}
