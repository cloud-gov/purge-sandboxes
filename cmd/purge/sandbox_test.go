package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/google/go-cmp/cmp"
)

func TestListRecipients(t *testing.T) {
	email1 := "foo1@bar.gov"
	email2 := "foo2@bar.gov"
	email3 := "foo3@bar.gov"
	emptyUsername := ""
	testCases := map[string]struct {
		userGUIDs          map[string]bool
		users              []*resource.User
		expectedRecipients []string
		expectedErr        string
	}{
		"skips users not in GUIDs map": {
			userGUIDs: map[string]bool{
				"user-1": true,
				"user-2": true,
			},
			users: []*resource.User{
				{Resource: resource.Resource{GUID: "user-1"}, Username: &email1},
				{Resource: resource.Resource{GUID: "user-2"}, Username: &email2},
				{Resource: resource.Resource{GUID: "user-3"}, Username: &email3},
			},
			expectedRecipients: []string{email1, email2},
		},
		"returns error for nil username": {
			userGUIDs: map[string]bool{
				"user-1": true,
			},
			users: []*resource.User{
				{Resource: resource.Resource{GUID: "user-1"}},
			},
			expectedErr: "username is required",
		},
		"returns error for missing username": {
			userGUIDs: map[string]bool{
				"user-1": true,
			},
			users: []*resource.User{
				{
					Resource: resource.Resource{GUID: "user-1"},
					Username: &emptyUsername,
				},
			},
			expectedErr: "mail: no address",
		},
	}
	for name, test := range testCases {
		t.Run(name, func(t *testing.T) {
			recipients, err := listRecipients(test.userGUIDs, test.users)
			if (test.expectedErr == "" && err != nil) || (test.expectedErr != "" && test.expectedErr != err.Error()) {
				t.Fatalf("expected error: %s, got: %s", test.expectedErr, err)
			}
			if diff := cmp.Diff(test.expectedRecipients, recipients); diff != "" {
				t.Errorf("ListRecipients() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestListSpaceDevsAndManagers(t *testing.T) {
	email1 := "foo1@bar.gov"
	email2 := "foo2@bar.gov"
	testCases := map[string]struct {
		userGUIDs        map[string]bool
		roles            []*resource.Role
		users            []*resource.User
		expectedDevs     []spaceUser
		expectedManagers []spaceUser
		expectedErr      string
	}{
		"returns correct devs and managers": {
			userGUIDs: map[string]bool{
				"user-1": true,
				"user-2": true,
			},
			users: []*resource.User{
				{
					Resource: resource.Resource{GUID: "user-1"},
					Username: &email1,
				},
				{
					Resource: resource.Resource{GUID: "user-2"},
					Username: &email2,
				},
			},
			roles: []*resource.Role{
				{
					Type: "space_developer",
					Relationships: resource.RoleSpaceUserOrganizationRelationships{
						User: resource.ToOneRelationship{
							Data: &resource.Relationship{
								GUID: "user-1",
							},
						},
					},
				},
				{
					Type: "space_manager",
					Relationships: resource.RoleSpaceUserOrganizationRelationships{
						User: resource.ToOneRelationship{
							Data: &resource.Relationship{
								GUID: "user-1",
							},
						},
					},
				},
				{
					Type: "space_developer",
					Relationships: resource.RoleSpaceUserOrganizationRelationships{
						User: resource.ToOneRelationship{
							Data: &resource.Relationship{
								GUID: "user-2",
							},
						},
					},
				},
			},
			expectedDevs: []spaceUser{
				{
					GUID:     "user-1",
					Username: email1,
				},
				{
					GUID:     "user-2",
					Username: email2,
				},
			},
			expectedManagers: []spaceUser{
				{
					GUID:     "user-1",
					Username: email1,
				},
			},
		},
		"skips users not in user GUIDs map": {
			userGUIDs: map[string]bool{
				"user-1": true,
			},
			users: []*resource.User{
				{
					Resource: resource.Resource{GUID: "user-1"},
					Username: &email1,
				},
				{
					Resource: resource.Resource{GUID: "user-2"},
					Username: &email2,
				},
			},
			roles: []*resource.Role{
				{
					Type: "space_developer",
					Relationships: resource.RoleSpaceUserOrganizationRelationships{
						User: resource.ToOneRelationship{
							Data: &resource.Relationship{
								GUID: "user-1",
							},
						},
					},
				},
				{
					Type: "space_developer",
					Relationships: resource.RoleSpaceUserOrganizationRelationships{
						User: resource.ToOneRelationship{
							Data: &resource.Relationship{
								GUID: "user-2",
							},
						},
					},
				},
			},
			expectedDevs: []spaceUser{
				{
					GUID:     "user-1",
					Username: email1,
				},
			},
			expectedManagers: []spaceUser{},
		},
		"skips users without username": {
			userGUIDs: map[string]bool{
				"user-1": true,
				"user-2": true,
			},
			users: []*resource.User{
				{
					Resource: resource.Resource{GUID: "user-1"},
					Username: &email1,
				},
			},
			roles: []*resource.Role{
				{
					Type: "space_developer",
					Relationships: resource.RoleSpaceUserOrganizationRelationships{
						User: resource.ToOneRelationship{
							Data: &resource.Relationship{
								GUID: "user-1",
							},
						},
					},
				},
				{
					Type: "space_developer",
					Relationships: resource.RoleSpaceUserOrganizationRelationships{
						User: resource.ToOneRelationship{
							Data: &resource.Relationship{
								GUID: "user-2",
							},
						},
					},
				},
			},
			expectedDevs: []spaceUser{
				{
					GUID:     "user-1",
					Username: email1,
				},
			},
			expectedManagers: []spaceUser{},
		},
	}
	for name, test := range testCases {
		t.Run(name, func(t *testing.T) {
			devs, managers := listSpaceDevsAndManagers(test.userGUIDs, test.roles, test.users)
			if diff := cmp.Diff(test.expectedDevs, devs); diff != "" {
				t.Errorf("ListSpaceDevsAndManagers() developers mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(test.expectedManagers, managers); diff != "" {
				t.Errorf("ListSpaceDevsAndManagers() managers mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestListPurgeSpaces(t *testing.T) {
	now := time.Now()
	testCases := map[string]struct {
		spaces           []*resource.Space
		apps             []*resource.App
		instances        []*resource.ServiceInstance
		now              time.Time
		expectedToNotify []SpaceDetails
		expectedToPurge  []SpaceDetails
		notifyThreshold  int
		purgeThreshold   int
		opts             Options
		expectedErr      string
		timeStartsAt     time.Time
	}{
		"skips empty spaces": {
			spaces: []*resource.Space{
				{
					Resource: resource.Resource{GUID: "space-guid"},
				},
			},
			now: now.Truncate(24 * time.Hour),
			opts: Options{
				NotifyDays: 25,
				PurgeDays:  30,
			},
			timeStartsAt: time.Time{},
		},
		"skips spaces with recent resources": {
			spaces: []*resource.Space{
				{
					Resource: resource.Resource{GUID: "space-guid"},
				},
			},
			now: now.Truncate(24 * time.Hour),
			apps: []*resource.App{
				{
					Resource: resource.Resource{GUID: "app-guid", CreatedAt: now.Add(-15 * 24 * time.Hour)},
					Relationships: resource.AppRelationships{
						Space: resource.ToOneRelationship{
							Data: &resource.Relationship{
								GUID: "space-guid",
							},
						},
					},
				},
			},
			opts: Options{
				NotifyDays: 25,
				PurgeDays:  30,
			},
			timeStartsAt: time.Time{},
		},
		"notifies on spaces between thresholds": {
			spaces: []*resource.Space{
				{
					Resource: resource.Resource{GUID: "space-guid"},
				},
			},
			now: now.Truncate(24 * time.Hour),
			apps: []*resource.App{
				{
					Resource: resource.Resource{GUID: "app-guid", CreatedAt: now.Add(-28 * 24 * time.Hour)},
					Relationships: resource.AppRelationships{
						Space: resource.ToOneRelationship{
							Data: &resource.Relationship{
								GUID: "space-guid",
							},
						},
					},
				},
			},
			opts: Options{
				NotifyDays: 25,
				PurgeDays:  30,
			},
			timeStartsAt: time.Time{},
			expectedToNotify: []SpaceDetails{
				{
					Timestamp: now.Add(-28 * 24 * time.Hour).Truncate(24 * time.Hour),
					Space: &resource.Space{
						Resource: resource.Resource{GUID: "space-guid"},
					},
				},
			},
		},
		"notifies on the notify threshold": {
			spaces: []*resource.Space{
				{
					Resource: resource.Resource{GUID: "space-guid"},
				},
			},
			now: now.Truncate(24 * time.Hour),
			apps: []*resource.App{
				{
					Resource: resource.Resource{GUID: "app-guid", CreatedAt: now.Add(-25 * 24 * time.Hour)},
					Relationships: resource.AppRelationships{
						Space: resource.ToOneRelationship{
							Data: &resource.Relationship{
								GUID: "space-guid",
							},
						},
					},
				},
			},
			opts: Options{
				NotifyDays: 25,
				PurgeDays:  30,
			},
			timeStartsAt: time.Time{},
			expectedToNotify: []SpaceDetails{
				{
					Timestamp: now.Add(-25 * 24 * time.Hour).Truncate(24 * time.Hour),
					Space: &resource.Space{
						Resource: resource.Resource{GUID: "space-guid"},
					},
				},
			},
		},
		"purges on the purge threshold": {
			spaces: []*resource.Space{
				{
					Resource: resource.Resource{GUID: "space-guid"},
				},
			},
			now: now.Truncate(24 * time.Hour),
			apps: []*resource.App{
				{
					Resource: resource.Resource{GUID: "app-guid", CreatedAt: now.Add(-30 * 24 * time.Hour)},
					Relationships: resource.AppRelationships{
						Space: resource.ToOneRelationship{
							Data: &resource.Relationship{
								GUID: "space-guid",
							},
						},
					},
				},
			},
			opts: Options{
				NotifyDays: 25,
				PurgeDays:  30,
			},
			timeStartsAt: time.Time{},
			expectedToPurge: []SpaceDetails{
				{
					Timestamp: now.Add(-30 * 24 * time.Hour).Truncate(24 * time.Hour),
					Space: &resource.Space{
						Resource: resource.Resource{GUID: "space-guid"},
					},
				},
			},
		},
		"purges after the purge threshold": {
			spaces: []*resource.Space{
				{
					Resource: resource.Resource{GUID: "space-guid"},
				},
			},
			now: now.Truncate(24 * time.Hour),
			apps: []*resource.App{
				{
					Resource: resource.Resource{GUID: "app-guid", CreatedAt: now.Add(-31 * 24 * time.Hour)},
					Relationships: resource.AppRelationships{
						Space: resource.ToOneRelationship{
							Data: &resource.Relationship{
								GUID: "space-guid",
							},
						},
					},
				},
			},
			opts: Options{
				NotifyDays: 25,
				PurgeDays:  30,
			},
			timeStartsAt: time.Time{},
			expectedToPurge: []SpaceDetails{
				{
					Timestamp: now.Add(-31 * 24 * time.Hour).Truncate(24 * time.Hour),
					Space: &resource.Space{
						Resource: resource.Resource{GUID: "space-guid"},
					},
				},
			},
		},
		"purges after the purge threshold when time starts in the past": {
			spaces: []*resource.Space{
				{
					Resource: resource.Resource{GUID: "space-guid"},
				},
			},
			now: now.Truncate(24 * time.Hour),
			apps: []*resource.App{
				{
					Resource: resource.Resource{GUID: "app-guid", CreatedAt: now.Add(-31 * 24 * time.Hour)},
					Relationships: resource.AppRelationships{
						Space: resource.ToOneRelationship{
							Data: &resource.Relationship{
								GUID: "space-guid",
							},
						},
					},
				},
			},
			opts: Options{
				NotifyDays: 25,
				PurgeDays:  30,
			},
			timeStartsAt: now.Add(-60 * 24 * time.Hour),
			expectedToPurge: []SpaceDetails{
				{
					Timestamp: now.Add(-31 * 24 * time.Hour).Truncate(24 * time.Hour),
					Space: &resource.Space{
						Resource: resource.Resource{GUID: "space-guid"},
					},
				},
			},
		},
		"skips purge when time starts after last timestamp": {
			spaces: []*resource.Space{
				{
					Resource: resource.Resource{GUID: "space-guid"},
				},
			},
			now: now.Truncate(24 * time.Hour),
			apps: []*resource.App{
				{
					Resource: resource.Resource{GUID: "app-guid", CreatedAt: now.Add(-31 * 24 * time.Hour)},
					Relationships: resource.AppRelationships{
						Space: resource.ToOneRelationship{
							Data: &resource.Relationship{
								GUID: "space-guid",
							},
						},
					},
				},
			},
			opts: Options{
				NotifyDays: 25,
				PurgeDays:  30,
			},
			timeStartsAt: now,
		},
		"notifies when purge is disabled even if time is past purge threshold": {
			spaces: []*resource.Space{
				{Resource: resource.Resource{GUID: "space-guid"}},
			},
			now: now.Truncate(24 * time.Hour),
			apps: []*resource.App{
				{
					Resource: resource.Resource{GUID: "app-guid", CreatedAt: now.Add(-31 * 24 * time.Hour)},
					Relationships: resource.AppRelationships{
						Space: resource.ToOneRelationship{
							Data: &resource.Relationship{
								GUID: "space-guid",
							},
						},
					},
				},
			},
			opts: Options{
				NotifyDays:   25,
				PurgeDays:    30,
				DisablePurge: true,
			},
			timeStartsAt: time.Time{},
			expectedToNotify: []SpaceDetails{
				{
					Timestamp: now.Add(-31 * 24 * time.Hour).Truncate(24 * time.Hour),
					Space: &resource.Space{
						Resource: resource.Resource{GUID: "space-guid"},
					},
				},
			},
		},
		"does not notify or purge when purge is disabled if time is past purge threshold but not notify threshold": {
			spaces: []*resource.Space{
				{
					Resource: resource.Resource{GUID: "space-guid"},
				},
			},
			now: now.Truncate(24 * time.Hour),
			apps: []*resource.App{
				{
					Resource: resource.Resource{GUID: "app-guid", CreatedAt: now.Add(-26 * 24 * time.Hour)},
					Relationships: resource.AppRelationships{
						Space: resource.ToOneRelationship{
							Data: &resource.Relationship{
								GUID: "space-guid",
							},
						},
					},
				},
			},
			opts: Options{
				NotifyDays:   30,
				PurgeDays:    25,
				DisablePurge: true,
			},
			timeStartsAt: time.Time{},
		},
	}
	for name, test := range testCases {
		t.Run(name, func(t *testing.T) {
			toNotify, toPurge, err := listPurgeSpaces(
				test.spaces,
				test.apps,
				test.instances,
				test.opts,
				test.now,
				test.timeStartsAt,
			)
			if (test.expectedErr == "" && err != nil) || (test.expectedErr != "" && test.expectedErr != err.Error()) {
				t.Fatalf("expected error: %s, got: %s", test.expectedErr, err)
			}
			if diff := cmp.Diff(test.expectedToNotify, toNotify); diff != "" {
				t.Errorf("ListPurgeSpaces() mismatch toNotify (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(test.expectedToPurge, toPurge); diff != "" {
				t.Errorf("ListPurgeSpaces() mismatch toPurge (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGetFirstResource(t *testing.T) {
	now := time.Now()
	ten_days_ago := now.Add(-10 * 24 * time.Hour)
	five_days_ago := now.Add(-5 * 24 * time.Hour)

	testCases := map[string]struct {
		space                 *resource.Space
		apps                  []*resource.App
		instances             []*resource.ServiceInstance
		expectedFirstResource time.Time
		expectedErr           string
	}{
		"skips empty spaces": {
			space: &resource.Space{
				Resource: resource.Resource{GUID: "space-guid"},
			},
		},
		"returns the timestamp of the earliest app": {
			space: &resource.Space{
				Resource: resource.Resource{GUID: "space-guid"},
			},
			apps: []*resource.App{
				{
					Resource: resource.Resource{GUID: "app-guid", CreatedAt: ten_days_ago},
					Relationships: resource.AppRelationships{
						Space: resource.ToOneRelationship{
							Data: &resource.Relationship{
								GUID: "space-guid",
							},
						},
					},
				},
			},
			instances: []*resource.ServiceInstance{
				{
					Resource: resource.Resource{GUID: "instance-guid", CreatedAt: five_days_ago},
					Relationships: resource.ServiceInstanceRelationships{
						Space: &resource.ToOneRelationship{
							Data: &resource.Relationship{
								GUID: "space-guid",
							},
						},
					},
				},
			},
			expectedFirstResource: ten_days_ago,
		},
		"returns the timestamp of the earliest instance": {
			space: &resource.Space{
				Resource: resource.Resource{GUID: "space-guid"},
			},
			apps: []*resource.App{
				{
					Resource: resource.Resource{GUID: "app-guid", CreatedAt: five_days_ago},
					Relationships: resource.AppRelationships{
						Space: resource.ToOneRelationship{
							Data: &resource.Relationship{
								GUID: "space-guid",
							},
						},
					},
				},
			},
			instances: []*resource.ServiceInstance{
				{
					Resource: resource.Resource{GUID: "instance-guid", CreatedAt: ten_days_ago},
					Relationships: resource.ServiceInstanceRelationships{
						Space: &resource.ToOneRelationship{
							Data: &resource.Relationship{
								GUID: "space-guid",
							},
						},
					},
				},
			},
			expectedFirstResource: ten_days_ago,
		},
	}
	for name, test := range testCases {
		t.Run(name, func(t *testing.T) {
			firstResource, err := getFirstResource(
				test.space,
				test.apps,
				test.instances,
			)
			if (test.expectedErr == "" && err != nil) || (test.expectedErr != "" && test.expectedErr != err.Error()) {
				t.Fatalf("expected error: %s, got: %s", test.expectedErr, err)
			}
			if !cmp.Equal(test.expectedFirstResource, firstResource) {
				t.Errorf("GetFirstResource() expected: %s, got: %s", test.expectedFirstResource, firstResource)
			}
		})
	}
}

func TestGetRetrySettingsDefaults(t *testing.T) {
	maxRetries, retryDelay, err := getRetrySettings()
	if err != nil {
		t.Fatal(err)
	}

	if maxRetries != int64(60) {
		t.Fail()
	}

	if retryDelay != int64(60) {
		t.Fail()
	}
}

func TestGetRetrySettingsFromEnvironment(t *testing.T) {
	t.Setenv("MAX_CF_POLL_RETRIES", "10")
	t.Setenv("CF_POLL_RETRY_DELAY", "5")

	maxRetries, retryDelay, err := getRetrySettings()
	if err != nil {
		t.Fatal(err)
	}

	if maxRetries != int64(10) {
		t.Fail()
	}

	if retryDelay != int64(5) {
		t.Fail()
	}
}

func TestWaitForServiceDeletion(t *testing.T) {
	testCases := map[string]struct {
		cfClient                            *cfResourceClient
		service                             *resource.ServiceInstance
		expectErr                           bool
		maxRetries                          int64
		retryDelay                          int64
		expectedGetServiceInstanceCallCount int64
	}{
		"success": {
			cfClient: &cfResourceClient{
				ServiceInstances: &mockServiceInstances{
					getServiceInstanceErr: resource.NewResourceNotFoundError(),
				},
			},
			service: &resource.ServiceInstance{
				Resource: resource.Resource{GUID: "service-1"},
			},
			maxRetries:                          1,
			retryDelay:                          0,
			expectedGetServiceInstanceCallCount: 1,
		},
		"error deleting service instance": {
			cfClient: &cfResourceClient{
				ServiceInstances: &mockServiceInstances{
					deleteServiceInstanceErr: errors.New("fail"),
				},
			},
			service: &resource.ServiceInstance{
				Resource: resource.Resource{GUID: "service-1"},
			},
			maxRetries: 1,
			retryDelay: 0,
			expectErr:  true,
		},
		"unexpected error fetching service instance": {
			cfClient: &cfResourceClient{
				ServiceInstances: &mockServiceInstances{
					getServiceInstanceErr: errors.New("fail"),
				},
			},
			service: &resource.ServiceInstance{
				Resource: resource.Resource{GUID: "service-1"},
			},
			maxRetries:                          1,
			retryDelay:                          0,
			expectErr:                           true,
			expectedGetServiceInstanceCallCount: 1,
		},
		"gives up after maximum retries waiting for deletion to complete": {
			cfClient: &cfResourceClient{
				ServiceInstances: &mockServiceInstances{},
			},
			service: &resource.ServiceInstance{
				Resource: resource.Resource{GUID: "service-1"},
			},
			maxRetries:                          5,
			retryDelay:                          0,
			expectErr:                           true,
			expectedGetServiceInstanceCallCount: 5,
		},
	}

	for name, test := range testCases {
		t.Run(name, func(t *testing.T) {
			err := waitForServiceInstanceDeletion(
				context.Background(),
				test.cfClient,
				test.service,
				test.maxRetries,
				test.retryDelay,
			)
			if !test.expectErr && err != nil {
				t.Fatal(err)
			}
			if test.expectErr && err == nil {
				t.Fatal("expected error but got none")
			}
			if mockServiceInstancesClient, ok := test.cfClient.ServiceInstances.(*mockServiceInstances); ok {
				if test.expectedGetServiceInstanceCallCount != int64(mockServiceInstancesClient.getServiceInstanceCallCount) {
					t.Fatalf("expected %d, got %d", test.expectedGetServiceInstanceCallCount, mockServiceInstancesClient.getServiceInstanceCallCount)
				}
			}
		})
	}
}

func TestPurgeSpace(t *testing.T) {
	deleteSpaceErr := errors.New("delete space error")
	listAppsErr := errors.New("error listing applications")
	deleteAppErr := errors.New("delete app error")

	testCases := map[string]struct {
		cfClient              *cfResourceClient
		space                 *resource.Space
		expectedErr           error
		expectedDeleteJobGUID string
		expectDeleteCallCount int
	}{
		"success": {
			cfClient: &cfResourceClient{
				Spaces: &mockSpaces{
					deleteJobGUID: "delete-1",
				},
				Applications:     &mockApplications{},
				ServiceInstances: &mockServiceInstances{},
			},
			space: &resource.Space{
				Resource: resource.Resource{GUID: "space-1"},
			},
			expectedDeleteJobGUID: "delete-1",
		},
		"success with deletion of service instance": {
			cfClient: &cfResourceClient{
				Spaces: &mockSpaces{
					deleteJobGUID: "delete-1",
				},
				Applications: &mockApplications{},
				ServiceInstances: &mockServiceInstances{
					listAllServiceInstances: []*resource.ServiceInstance{
						{
							Resource: resource.Resource{GUID: "service-1"},
						},
					},
					getServiceInstanceErr: resource.NewResourceNotFoundError(),
				},
			},
			space: &resource.Space{
				Resource: resource.Resource{GUID: "space-1"},
			},
			expectedDeleteJobGUID: "delete-1",
		},
		"error deleting space": {
			cfClient: &cfResourceClient{
				Spaces: &mockSpaces{
					deleteErr: deleteSpaceErr,
				},
				Applications: &mockApplications{
					apps: []*resource.App{
						{
							Resource: resource.Resource{GUID: "app-1"},
						},
					},
				},
				ServiceInstances: &mockServiceInstances{},
			},
			space: &resource.Space{
				Resource: resource.Resource{GUID: "space-1"},
			},
			expectedErr:           deleteSpaceErr,
			expectDeleteCallCount: 1,
		},
		"error listing applications": {
			cfClient: &cfResourceClient{
				Spaces: &mockSpaces{
					deleteErr: deleteSpaceErr,
				},
				Applications: &mockApplications{
					listAppsErr: listAppsErr,
				},
				ServiceInstances: &mockServiceInstances{},
			},
			space: &resource.Space{
				Resource: resource.Resource{GUID: "space-1"},
			},
			expectedErr: listAppsErr,
		},
		"error deleting applications": {
			cfClient: &cfResourceClient{
				Spaces: &mockSpaces{
					deleteErr: deleteSpaceErr,
				},
				Applications: &mockApplications{
					apps: []*resource.App{
						{
							Resource: resource.Resource{GUID: "app-1"},
						},
					},
					deleteErr: deleteAppErr,
				},
				ServiceInstances: &mockServiceInstances{},
			},
			space: &resource.Space{
				Resource: resource.Resource{GUID: "space-1"},
			},
			expectDeleteCallCount: 1,
			expectedErr:           deleteAppErr,
		},
	}

	for name, test := range testCases {
		t.Run(name, func(t *testing.T) {
			deleteJobGUID, err := purgeSpace(
				context.Background(),
				test.cfClient,
				test.space,
			)

			if deleteJobGUID != test.expectedDeleteJobGUID {
				t.Fatalf("expected delete job GUID: %s, got: %s", test.expectedDeleteJobGUID, deleteJobGUID)
			}

			if mockApps, ok := test.cfClient.Applications.(*mockApplications); ok {
				if mockApps.deleteCallCount != test.expectDeleteCallCount {
					t.Fatalf("expected app delete call count: %d, got: %d", test.expectDeleteCallCount, mockApps.deleteCallCount)
				}
			}

			if !errors.Is(err, test.expectedErr) {
				t.Fatalf("expected error: %s, got: %s", test.expectedErr, err)
			}
		})
	}
}
