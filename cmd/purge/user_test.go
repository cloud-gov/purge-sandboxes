package main

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/google/go-cmp/cmp"
)

func TestGetValidUsersMap(t *testing.T) {
	email := "test@example.gov"
	testCases := map[string]struct {
		cfClient              *cfResourceClient
		expectedValidUsersMap map[string]bool
		expectErr             bool
	}{
		"returns expected map": {
			cfClient: &cfResourceClient{
				Users: &mockUsers{
					users: []*resource.User{
						{
							Resource: resource.Resource{GUID: "user-1"},
							Username: &email,
						},
					},
				},
			},
			expectedValidUsersMap: map[string]bool{
				"user-1": true,
			},
		},
		"returns empty map if user has no email": {
			cfClient: &cfResourceClient{
				Users: &mockUsers{
					users: []*resource.User{
						{
							Resource: resource.Resource{GUID: "user-1"},
						},
					},
				},
			},
			expectedValidUsersMap: map[string]bool{},
		},
		"returns error": {
			cfClient: &cfResourceClient{
				Users: &mockUsers{
					listAllUsersErr: errors.New("failed to list users"),
				},
			},
			expectErr: true,
		},
	}

	for name, test := range testCases {
		t.Run(name, func(t *testing.T) {
			validUsersMap, err := getValidUsersMap(context.Background(), test.cfClient)
			if err != nil && !test.expectErr {
				t.Fatal(err)
			}
			if diff := cmp.Diff(test.expectedValidUsersMap, validUsersMap); diff != "" {
				t.Errorf("getValidUsersMap(): (-want +got):\n%s", diff)
			}
		})
	}
}
