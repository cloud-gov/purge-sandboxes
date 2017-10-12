package sandbox_test

import (
	"time"

	"github.com/cloudfoundry-community/go-cfclient"

	"github.com/18F/cg-sandbox/sandbox"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Sandbox", func() {
	Describe("ListPurgeSpaces", func() {
		var (
			spaces    []cfclient.Space
			apps      []cfclient.App
			instances []cfclient.ServiceInstance
			now       time.Time
		)

		BeforeEach(func() {
			spaces = []cfclient.Space{
				{Guid: "space-guid"},
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
			apps = []cfclient.App{
				{
					Guid:      "app-guid",
					SpaceGuid: "space-guid",
					CreatedAt: now.Add(-15 * 24 * time.Hour).Format(time.RFC3339Nano),
				},
			}
			toNotify, toPurge, err := sandbox.ListPurgeSpaces(spaces, apps, instances, now, 25, 30, time.Time{})
			Expect(err).NotTo(HaveOccurred())
			Expect(toNotify).To(HaveLen(0))
			Expect(toPurge).To(HaveLen(0))
		})

		It("notifies on spaces between thresholds", func() {
			apps = []cfclient.App{
				{
					Guid:      "app-guid",
					SpaceGuid: "space-guid",
					CreatedAt: now.Add(-28 * 24 * time.Hour).Format(time.RFC3339Nano),
				},
			}
			toNotify, toPurge, err := sandbox.ListPurgeSpaces(spaces, apps, instances, now, 25, 30, time.Time{})
			Expect(err).NotTo(HaveOccurred())
			Expect(toNotify).To(HaveLen(1))
			Expect(toPurge).To(HaveLen(0))
		})

		It("notifies on the notify threshold", func() {
			apps = []cfclient.App{
				{
					Guid:      "app-guid",
					SpaceGuid: "space-guid",
					CreatedAt: now.Add(-25 * 24 * time.Hour).Format(time.RFC3339Nano),
				},
			}
			toNotify, toPurge, err := sandbox.ListPurgeSpaces(spaces, apps, instances, now, 25, 30, time.Time{})
			Expect(err).NotTo(HaveOccurred())
			Expect(toNotify).To(HaveLen(1))
			Expect(toPurge).To(HaveLen(0))
		})

		It("purges on the purge threshold", func() {
			apps = []cfclient.App{
				{
					Guid:      "app-guid",
					SpaceGuid: "space-guid",
					CreatedAt: now.Add(-30 * 24 * time.Hour).Format(time.RFC3339Nano),
				},
			}
			toNotify, toPurge, err := sandbox.ListPurgeSpaces(spaces, apps, instances, now, 25, 30, time.Time{})
			Expect(err).NotTo(HaveOccurred())
			Expect(toNotify).To(HaveLen(0))
			Expect(toPurge).To(HaveLen(1))
		})

		It("purges after the purge threshold", func() {
			apps = []cfclient.App{
				{
					Guid:      "app-guid",
					SpaceGuid: "space-guid",
					CreatedAt: now.Add(-31 * 24 * time.Hour).Format(time.RFC3339Nano),
				},
			}
			toNotify, toPurge, err := sandbox.ListPurgeSpaces(spaces, apps, instances, now, 25, 30, time.Time{})
			Expect(err).NotTo(HaveOccurred())
			Expect(toNotify).To(HaveLen(0))
			Expect(toPurge).To(HaveLen(1))
		})

		It("purges after the purge threshold when time starts in the past", func() {
			apps = []cfclient.App{
				{
					Guid:      "app-guid",
					SpaceGuid: "space-guid",
					CreatedAt: now.Add(-31 * 24 * time.Hour).Format(time.RFC3339Nano),
				},
			}
			toNotify, toPurge, err := sandbox.ListPurgeSpaces(spaces, apps, instances, now, 25, 30, now.Add(-60*24*time.Hour))
			Expect(err).NotTo(HaveOccurred())
			Expect(toNotify).To(HaveLen(0))
			Expect(toPurge).To(HaveLen(1))
		})

		It("skips purge when time starts after last timestamp", func() {
			apps = []cfclient.App{
				{
					Guid:      "app-guid",
					SpaceGuid: "space-guid",
					CreatedAt: now.Add(-31 * 24 * time.Hour).Format(time.RFC3339Nano),
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
			space     cfclient.Space
			apps      []cfclient.App
			instances []cfclient.ServiceInstance
		)

		BeforeEach(func() {
			space = cfclient.Space{
				Guid: "space-guid",
			}
		})

		It("returns the zero value for an empty space", func() {
			firstResource, err := sandbox.GetFirstResource(space, apps, instances)
			Expect(err).NotTo(HaveOccurred())
			Expect(firstResource.IsZero()).To(BeTrue())
		})

		It("returns the timestamp of the earliest app", func() {
			now := time.Now()
			apps = []cfclient.App{
				{
					Guid:      "app-guid",
					SpaceGuid: "space-guid",
					CreatedAt: now.Add(-10 * 24 * time.Hour).Format(time.RFC3339Nano),
				},
			}
			instances = []cfclient.ServiceInstance{
				{
					Guid:      "instance-guid",
					SpaceGuid: "space-guid",
					CreatedAt: now.Add(-5 * 24 * time.Hour).Format(time.RFC3339Nano),
				},
			}
			firstResource, err := sandbox.GetFirstResource(space, apps, instances)
			Expect(err).NotTo(HaveOccurred())
			firstResource.Equal(now.Add(-10 * 24 * time.Hour))
		})

		It("returns the timestamp of the earliest instance", func() {
			now := time.Now()
			apps = []cfclient.App{
				{
					Guid:      "app-guid",
					SpaceGuid: "space-guid",
					CreatedAt: now.Add(-5 * 24 * time.Hour).Format(time.RFC3339Nano),
				},
			}
			instances = []cfclient.ServiceInstance{
				{
					Guid:      "instance-guid",
					SpaceGuid: "space-guid",
					CreatedAt: now.Add(-10 * 24 * time.Hour).Format(time.RFC3339Nano),
				},
			}
			firstResource, err := sandbox.GetFirstResource(space, apps, instances)
			Expect(err).NotTo(HaveOccurred())
			firstResource.Equal(now.Add(-10 * 24 * time.Hour))
		})
	})
})
