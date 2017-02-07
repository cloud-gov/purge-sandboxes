package main

import (
	"log"

	"github.com/cloudfoundry-community/go-cfclient"
	"github.com/kelseyhightower/envconfig"
)

type Options struct {
	APIAddress   string `envconfig:"api_address" required:"true"`
	ClientID     string `envconfig:"client_id" required:"true"`
	ClientSecret string `envconfig:"client_secret" required:"true"`
	OrgName      string `envconfig:"org_name" required:"true"`
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

	err = client.DeleteOrg(org.Guid)
	if err != nil {
		log.Fatalf("error deleting org: %s", err.Error())
	}
}
