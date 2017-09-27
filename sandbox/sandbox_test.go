package sandbox_test

import (
	"time"

	"github.com/cloudfoundry-community/go-cfclient"

	"github.com/18F/cg-sandbox/sandbox"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Sandbox", func() {
	Describe("ListNotifySpaces", func() {
		var (
			spaces    []cfclient.Space
			apps      []cfclient.App
			instances []cfclient.ServiceInstance
		)

		It("skips recently created spaces", func() {
			spaces = []cfclient.Space{
				{
					Guid:      "space-guid",
					CreatedAt: time.Now().Format(time.RFC3339Nano),
				},
			}
			apps = []cfclient.App{
				{
					Guid:      "app-guid",
					SpaceGuid: "space-guid",
				},
			}
			notifySpaces, err := sandbox.ListNotifySpaces(spaces, apps, instances, 25)
			Expect(err).NotTo(HaveOccurred())
			Expect(notifySpaces).To(HaveLen(0))
		})

		It("skips empty spaces", func() {
			spaces = []cfclient.Space{
				{
					Guid:      "space-guid",
					CreatedAt: time.Now().Format(time.RFC3339Nano),
				},
			}
			notifySpaces, err := sandbox.ListNotifySpaces(spaces, apps, instances, 25)
			Expect(err).NotTo(HaveOccurred())
			Expect(notifySpaces).To(HaveLen(0))
		})

		It("notifies on a recent space with an app", func() {
			spaces = []cfclient.Space{
				{
					Guid:      "space-guid",
					CreatedAt: time.Now().Add(-30 * 24 * time.Hour).Format(time.RFC3339Nano),
				},
			}
			apps = []cfclient.App{
				{
					Guid:      "app-guid",
					SpaceGuid: "space-guid",
				},
			}
			notifySpaces, err := sandbox.ListNotifySpaces(spaces, apps, instances, 25)
			Expect(err).NotTo(HaveOccurred())
			Expect(notifySpaces).To(HaveLen(1))
		})

		It("notifies on a recent space with a service instance", func() {
			spaces = []cfclient.Space{
				{
					Guid:      "space-guid",
					CreatedAt: time.Now().Add(-30 * 24 * time.Hour).Format(time.RFC3339Nano),
				},
			}
			instances = []cfclient.ServiceInstance{
				{
					Guid:      "instance-guid",
					SpaceGuid: "space-guid",
				},
			}
			notifySpaces, err := sandbox.ListNotifySpaces(spaces, apps, instances, 25)
			Expect(err).NotTo(HaveOccurred())
			Expect(notifySpaces).To(HaveLen(1))
		})
	})

	Describe("ListNotifySpaces", func() {
		var (
			spaces    []cfclient.Space
			apps      []cfclient.App
			instances []cfclient.ServiceInstance
		)

		It("purges a space with an old app", func() {
			spaces = []cfclient.Space{
				{
					Guid:      "space-guid",
					CreatedAt: time.Now().Add(-60 * 24 * time.Hour).Format(time.RFC3339Nano),
				},
			}
			apps = []cfclient.App{
				{
					Guid:      "app-guid",
					SpaceGuid: "space-guid",
					CreatedAt: time.Now().Add(-30 * 24 * time.Hour).Format(time.RFC3339Nano),
				},
			}
			purgeSpaces, err := sandbox.ListPurgeSpaces(spaces, apps, instances, 30, 5)
			Expect(err).NotTo(HaveOccurred())
			Expect(purgeSpaces).To(HaveLen(1))
		})

		It("purges a space with an old instance", func() {
			spaces = []cfclient.Space{
				{
					Guid:      "space-guid",
					CreatedAt: time.Now().Add(-60 * 24 * time.Hour).Format(time.RFC3339Nano),
				},
			}
			instances = []cfclient.ServiceInstance{
				{
					Guid:      "app-guid",
					SpaceGuid: "space-guid",
					CreatedAt: time.Now().Add(-30 * 24 * time.Hour).Format(time.RFC3339Nano),
				},
			}
			purgeSpaces, err := sandbox.ListPurgeSpaces(spaces, apps, instances, 30, 5)
			Expect(err).NotTo(HaveOccurred())
			Expect(purgeSpaces).To(HaveLen(1))
		})

		It("skips a space with a new app", func() {
			spaces = []cfclient.Space{
				{
					Guid:      "space-guid",
					CreatedAt: time.Now().Add(-60 * 24 * time.Hour).Format(time.RFC3339Nano),
				},
			}
			apps = []cfclient.App{
				{
					Guid:      "app-guid",
					SpaceGuid: "space-guid",
					CreatedAt: time.Now().Add(-1 * 24 * time.Hour).Format(time.RFC3339Nano),
				},
			}
			purgeSpaces, err := sandbox.ListPurgeSpaces(spaces, apps, instances, 30, 5)
			Expect(err).NotTo(HaveOccurred())
			Expect(purgeSpaces).To(HaveLen(0))
		})
	})
})
