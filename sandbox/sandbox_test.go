package sandbox_test

import (
	"html/template"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/cloudfoundry-community/go-cfclient/v3/resource"
	"github.com/google/go-cmp/cmp"

	"github.com/18f/cg-sandbox/sandbox"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestListRecipients(t *testing.T) {
	testCases := map[string]struct {
		userGUIDs          map[string]bool
		users              []*resource.User
		expectedRecipients []string
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
	}
	for name, test := range testCases {
		t.Run(name, func(t *testing.T) {
			recipients, err := sandbox.ListRecipients(test.userGUIDs, test.users)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			if diff := cmp.Diff(test.expectedRecipients, recipients); diff != "" {
				t.Errorf("ListRecipients() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

var _ = Describe("Sandbox", func() {
	Describe("ListRecipients", func() {
		var (
			userGUIDs map[string]bool
			users     []*resource.User
		)

		It("skips users not in guids map", func() {
			userGUIDs = map[string]bool{
				"user-1": true,
				"user-2": true,
			}
			users = []*resource.User{
				{GUID: "user-1"},
				{GUID: "user-2"},
				{GUID: "user-3"},
			}
			recipients, _ := sandbox.ListRecipients(userGUIDs, users)
			Expect(recipients).To(Equal([]string{"user-1", "user-2"}))
		})

		It("parses email addresses", func() {
			userGUIDs = map[string]bool{
				"user-1": true,
			}
			users = []*resource.User{
				{GUID: "user-1", Username: "foo@bar.gov"},
				{GUID: "user-2"},
			}
			recipients, _ := sandbox.ListRecipients(userGUIDs, users)
			Expect(recipients).To(Equal([]string{"foo@bar.gov"}))
		})
	})

	Describe("ListSpaceDevsAndManagers", func() {
		var (
			userGUIDs map[string]bool
			roles     []*resource.Role
		)

		It("skips users not in guids map", func() {
			userGUIDs = map[string]bool{
				"user-1": true,
				"user-2": true,
			}
			roles = []*resource.Role{
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
			}
			developers, managers := sandbox.ListSpaceDevsAndManagers(userGUIDs, roles)
			Expect(developers).To(Equal([]string{"user-1", "user-2"}))
			Expect(managers).To(Equal([]string{"user-1"}))
		})
	})

	Describe("ListPurgeSpaces", func() {
		var (
			spaces    []*resource.Space
			apps      []*resource.App
			instances []*resource.ServiceInstance
			now       time.Time
		)

		BeforeEach(func() {
			spaces = []*resource.Space{
				{GUID: "space-guid"},
			}
			now = time.Now().Truncate(24 * time.Hour)
		})

		It("skips empty spaces", func() {
			toNotify, toPurge, err := sandbox.ListPurgeSpaces(spaces, apps, instances, now, 25, 30, time.Time{})
			Expect(err).NotTo(HaveOccurred())
			Expect(toNotify).To(HaveLen(0))
			Expect(toPurge).To(HaveLen(0))
		})

		It("skips spaces with recent resources", func() {
			apps = []*resource.App{
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
			}
			toNotify, toPurge, err := sandbox.ListPurgeSpaces(spaces, apps, instances, now, 25, 30, time.Time{})
			Expect(err).NotTo(HaveOccurred())
			Expect(toNotify).To(HaveLen(0))
			Expect(toPurge).To(HaveLen(0))
		})

		It("notifies on spaces between thresholds", func() {
			apps = []*resource.App{
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
			}
			toNotify, toPurge, err := sandbox.ListPurgeSpaces(spaces, apps, instances, now, 25, 30, time.Time{})
			Expect(err).NotTo(HaveOccurred())
			Expect(toNotify).To(HaveLen(1))
			Expect(toPurge).To(HaveLen(0))
		})

		It("notifies on the notify threshold", func() {
			apps = []*resource.App{
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
			}
			toNotify, toPurge, err := sandbox.ListPurgeSpaces(spaces, apps, instances, now, 25, 30, time.Time{})
			Expect(err).NotTo(HaveOccurred())
			Expect(toNotify).To(HaveLen(1))
			Expect(toPurge).To(HaveLen(0))
		})

		It("purges on the purge threshold", func() {
			apps = []*resource.App{
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
			}
			toNotify, toPurge, err := sandbox.ListPurgeSpaces(spaces, apps, instances, now, 25, 30, time.Time{})
			Expect(err).NotTo(HaveOccurred())
			Expect(toNotify).To(HaveLen(0))
			Expect(toPurge).To(HaveLen(1))
		})

		It("purges after the purge threshold", func() {
			apps = []*resource.App{
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
			}
			toNotify, toPurge, err := sandbox.ListPurgeSpaces(spaces, apps, instances, now, 25, 30, time.Time{})
			Expect(err).NotTo(HaveOccurred())
			Expect(toNotify).To(HaveLen(0))
			Expect(toPurge).To(HaveLen(1))
		})

		It("purges after the purge threshold when time starts in the past", func() {
			apps = []*resource.App{
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
			}
			toNotify, toPurge, err := sandbox.ListPurgeSpaces(spaces, apps, instances, now, 25, 30, now.Add(-60*24*time.Hour))
			Expect(err).NotTo(HaveOccurred())
			Expect(toNotify).To(HaveLen(0))
			Expect(toPurge).To(HaveLen(1))
		})

		It("skips purge when time starts after last timestamp", func() {
			apps = []*resource.App{
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
			}
			toNotify, toPurge, err := sandbox.ListPurgeSpaces(spaces, apps, instances, now, 25, 30, now)
			Expect(err).NotTo(HaveOccurred())
			Expect(toNotify).To(HaveLen(0))
			Expect(toPurge).To(HaveLen(0))
		})
	})

	Describe("GetFirstResource", func() {
		var (
			space     *resource.Space
			apps      []*resource.App
			instances []*resource.ServiceInstance
		)

		BeforeEach(func() {
			space = &resource.Space{
				GUID: "space-guid",
			}
		})

		It("returns the zero value for an empty space", func() {
			firstResource, err := sandbox.GetFirstResource(space, apps, instances)
			Expect(err).NotTo(HaveOccurred())
			Expect(firstResource.IsZero()).To(BeTrue())
		})

		It("returns the timestamp of the earliest app", func() {
			now := time.Now()
			apps = []*resource.App{
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
			}
			instances = []*resource.ServiceInstance{
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
			}
			firstResource, err := sandbox.GetFirstResource(space, apps, instances)
			Expect(err).NotTo(HaveOccurred())
			firstResource.Equal(now.Add(-10 * 24 * time.Hour))
		})

		It("returns the timestamp of the earliest instance", func() {
			now := time.Now()
			apps = []*resource.App{
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
			}
			instances = []*resource.ServiceInstance{
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
			}
			firstResource, err := sandbox.GetFirstResource(space, apps, instances)
			Expect(err).NotTo(HaveOccurred())
			firstResource.Equal(now.Add(-10 * 24 * time.Hour))
		})
	})
	Describe("RenderTemplate", func() {
		var (
			tpl              *template.Template
			data             map[string]interface{}
			err              error
			expectedTestFile string
		)
		Context("NotifyTemplate", func() {
			BeforeEach(func() {
				tpl, err = template.ParseFiles("../templates/base.html", "../templates/notify.tmpl")
				data = map[string]interface{}{
					"org": &resource.Organization{
						Name: "test-org",
					},
					"space": &resource.Space{
						Name: "test-space",
					},
					"date": time.Date(2009, 11, 17, 20, 34, 58, 651387237, time.UTC),
					"days": 90,
				}
				expectedTestFile = "../testdata/notify.html"
				Expect(err).NotTo(HaveOccurred())
			})
			It("constructs the appropriate notify template", func() {
				renderedTemplate, err := sandbox.RenderTemplate(tpl, data)
				Expect(err).NotTo(HaveOccurred())
				if os.Getenv("OVERRIDE_TEMPLATES") == "1" {
					err := ioutil.WriteFile(expectedTestFile, []byte(renderedTemplate), 0644)
					Expect(err).NotTo(HaveOccurred())
				}
				expected, err := ioutil.ReadFile(expectedTestFile)
				Expect(err).NotTo(HaveOccurred())
				Expect(renderedTemplate).To(Equal(string(expected)))
			})
		})
		Context("PurgeTemplate", func() {
			BeforeEach(func() {
				tpl, err = template.ParseFiles("../templates/base.html", "../templates/purge.tmpl")
				data = map[string]interface{}{
					"org": &resource.Organization{
						Name: "test-org",
					},
					"space": &resource.Space{
						Name: "test-space",
					},
					"date": time.Date(2009, 11, 17, 20, 34, 58, 651387237, time.UTC),
					"days": 90,
				}
				expectedTestFile = "../testdata/purge.html"
				Expect(err).NotTo(HaveOccurred())
			})
			It("constructs the appropriate notify template", func() {
				renderedTemplate, err := sandbox.RenderTemplate(tpl, data)
				Expect(err).NotTo(HaveOccurred())
				if os.Getenv("OVERRIDE_TEMPLATES") == "1" {
					err := ioutil.WriteFile(expectedTestFile, []byte(renderedTemplate), 0644)
					Expect(err).NotTo(HaveOccurred())
				}
				expected, err := ioutil.ReadFile(expectedTestFile)
				Expect(err).NotTo(HaveOccurred())
				Expect(renderedTemplate).To(Equal(string(expected)))
			})
		})
	})
})
