package main

import (
	"testing"
	"time"

	"github.com/cloudfoundry-community/go-cfclient/v3/resource"
	"github.com/google/go-cmp/cmp"
)

func TestListRecipients(t *testing.T) {
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
				{GUID: "user-1", Username: "foo1@bar.gov"},
				{GUID: "user-2", Username: "foo2@bar.gov"},
				{GUID: "user-3", Username: "foo3@bar.gov"},
			},
			expectedRecipients: []string{"foo1@bar.gov", "foo2@bar.gov"},
		},
		"returns error for missing username": {
			userGUIDs: map[string]bool{
				"user-1": true,
			},
			users: []*resource.User{
				{GUID: "user-1"},
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
	testCases := map[string]struct {
		userGUIDs        map[string]bool
		roles            []*resource.Role
		users            []*resource.User
		expectedDevs     []string
		expectedManagers []string
		expectedErr      string
	}{
		"returns correct devs and managers": {
			userGUIDs: map[string]bool{
				"user-1": true,
				"user-2": true,
			},
			users: []*resource.User{
				{
					GUID:     "user-1",
					Username: "foo1@bar.gov",
				},
				{
					GUID:     "user-2",
					Username: "foo2@bar.gov",
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
			expectedDevs:     []string{"foo1@bar.gov", "foo2@bar.gov"},
			expectedManagers: []string{"foo1@bar.gov"},
		},
		"skips users not in user GUIDs map": {
			userGUIDs: map[string]bool{
				"user-1": true,
			},
			users: []*resource.User{
				{
					GUID:     "user-1",
					Username: "foo1@bar.gov",
				},
				{
					GUID:     "user-2",
					Username: "foo2@bar.gov",
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
			expectedDevs:     []string{"foo1@bar.gov"},
			expectedManagers: []string{},
		},
		"skips users without username": {
			userGUIDs: map[string]bool{
				"user-1": true,
				"user-2": true,
			},
			users: []*resource.User{
				{
					GUID:     "user-1",
					Username: "foo1@bar.gov",
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
			expectedDevs:     []string{"foo1@bar.gov"},
			expectedManagers: []string{},
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
				{GUID: "space-guid"},
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
				{GUID: "space-guid"},
			},
			now: now.Truncate(24 * time.Hour),
			apps: []*resource.App{
				{
					GUID: "app-guid",
					Relationships: resource.SpaceRelationship{
						Space: resource.ToOneRelationship{
							Data: &resource.Relationship{
								GUID: "space-guid",
							},
						},
					},
					CreatedAt: now.Add(-15 * 24 * time.Hour),
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
				{GUID: "space-guid"},
			},
			now: now.Truncate(24 * time.Hour),
			apps: []*resource.App{
				{
					GUID: "app-guid",
					Relationships: resource.SpaceRelationship{
						Space: resource.ToOneRelationship{
							Data: &resource.Relationship{
								GUID: "space-guid",
							},
						},
					},
					CreatedAt: now.Add(-28 * 24 * time.Hour),
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
						GUID: "space-guid",
					},
				},
			},
		},
		"notifies on the notify threshold": {
			spaces: []*resource.Space{
				{GUID: "space-guid"},
			},
			now: now.Truncate(24 * time.Hour),
			apps: []*resource.App{
				{
					GUID: "app-guid",
					Relationships: resource.SpaceRelationship{
						Space: resource.ToOneRelationship{
							Data: &resource.Relationship{
								GUID: "space-guid",
							},
						},
					},
					CreatedAt: now.Add(-25 * 24 * time.Hour),
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
						GUID: "space-guid",
					},
				},
			},
		},
		"purges on the purge threshold": {
			spaces: []*resource.Space{
				{GUID: "space-guid"},
			},
			now: now.Truncate(24 * time.Hour),
			apps: []*resource.App{
				{
					GUID: "app-guid",
					Relationships: resource.SpaceRelationship{
						Space: resource.ToOneRelationship{
							Data: &resource.Relationship{
								GUID: "space-guid",
							},
						},
					},
					CreatedAt: now.Add(-30 * 24 * time.Hour),
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
						GUID: "space-guid",
					},
				},
			},
		},
		"purges after the purge threshold": {
			spaces: []*resource.Space{
				{GUID: "space-guid"},
			},
			now: now.Truncate(24 * time.Hour),
			apps: []*resource.App{
				{
					GUID: "app-guid",
					Relationships: resource.SpaceRelationship{
						Space: resource.ToOneRelationship{
							Data: &resource.Relationship{
								GUID: "space-guid",
							},
						},
					},
					CreatedAt: now.Add(-31 * 24 * time.Hour),
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
						GUID: "space-guid",
					},
				},
			},
		},
		"purges after the purge threshold when time starts in the past": {
			spaces: []*resource.Space{
				{GUID: "space-guid"},
			},
			now: now.Truncate(24 * time.Hour),
			apps: []*resource.App{
				{
					GUID: "app-guid",
					Relationships: resource.SpaceRelationship{
						Space: resource.ToOneRelationship{
							Data: &resource.Relationship{
								GUID: "space-guid",
							},
						},
					},
					CreatedAt: now.Add(-31 * 24 * time.Hour),
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
						GUID: "space-guid",
					},
				},
			},
		},
		"skips purge when time starts after last timestamp": {
			spaces: []*resource.Space{
				{GUID: "space-guid"},
			},
			now: now.Truncate(24 * time.Hour),
			apps: []*resource.App{
				{
					GUID: "app-guid",
					Relationships: resource.SpaceRelationship{
						Space: resource.ToOneRelationship{
							Data: &resource.Relationship{
								GUID: "space-guid",
							},
						},
					},
					CreatedAt: now.Add(-31 * 24 * time.Hour),
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
				{GUID: "space-guid"},
			},
			now: now.Truncate(24 * time.Hour),
			apps: []*resource.App{
				{
					GUID: "app-guid",
					Relationships: resource.SpaceRelationship{
						Space: resource.ToOneRelationship{
							Data: &resource.Relationship{
								GUID: "space-guid",
							},
						},
					},
					CreatedAt: now.Add(-31 * 24 * time.Hour),
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
						GUID: "space-guid",
					},
				},
			},
		},
		"does not notify or purge when purge is disabled if time is past purge threshold but not notify threshold": {
			spaces: []*resource.Space{
				{GUID: "space-guid"},
			},
			now: now.Truncate(24 * time.Hour),
			apps: []*resource.App{
				{
					GUID: "app-guid",
					Relationships: resource.SpaceRelationship{
						Space: resource.ToOneRelationship{
							Data: &resource.Relationship{
								GUID: "space-guid",
							},
						},
					},
					CreatedAt: now.Add(-26 * 24 * time.Hour),
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
	testCases := map[string]struct {
		space                 *resource.Space
		apps                  []*resource.App
		instances             []*resource.ServiceInstance
		expectedFirstResource time.Time
		expectedErr           string
	}{
		"skips empty spaces": {
			space: &resource.Space{
				GUID: "space-guid",
			},
		},
		"returns the timestamp of the earliest app": {
			space: &resource.Space{
				GUID: "space-guid",
			},
			apps: []*resource.App{
				{
					GUID: "app-guid",
					Relationships: resource.SpaceRelationship{
						Space: resource.ToOneRelationship{
							Data: &resource.Relationship{
								GUID: "space-guid",
							},
						},
					},
					CreatedAt: now.Add(-10 * 24 * time.Hour),
				},
			},
			instances: []*resource.ServiceInstance{
				{
					GUID: "instance-guid",
					Relationships: resource.ServiceInstanceRelationships{
						Space: &resource.ToOneRelationship{
							Data: &resource.Relationship{
								GUID: "space-guid",
							},
						},
					},
					CreatedAt: now.Add(-5 * 24 * time.Hour),
				},
			},
			expectedFirstResource: now.Add(-10 * 24 * time.Hour),
		},
		"returns the timestamp of the earliest instance": {
			space: &resource.Space{
				GUID: "space-guid",
			},
			apps: []*resource.App{
				{
					GUID: "app-guid",
					Relationships: resource.SpaceRelationship{
						Space: resource.ToOneRelationship{
							Data: &resource.Relationship{
								GUID: "space-guid",
							},
						},
					},
					CreatedAt: now.Add(-5 * 24 * time.Hour),
				},
			},
			instances: []*resource.ServiceInstance{
				{
					GUID: "instance-guid",
					Relationships: resource.ServiceInstanceRelationships{
						Space: &resource.ToOneRelationship{
							Data: &resource.Relationship{
								GUID: "space-guid",
							},
						},
					},
					CreatedAt: now.Add(-10 * 24 * time.Hour),
				},
			},
			expectedFirstResource: now.Add(-10 * 24 * time.Hour),
		},
	}
	for name, test := range testCases {
		t.Run(name, func(t *testing.T) {
			firstResource, err := letFirstResource(
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
