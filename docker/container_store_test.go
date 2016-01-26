package docker_test

import (
	. "github.com/swipely/iam-docker/docker"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ContainerStore", func() {
	const (
		id      = "DEADBEEF"
		ip      = "172.0.0.2"
		iamRole = "arn:aws:iam::0123456789:role/test-role"
	)
	var store ContainerStore

	BeforeEach(func() {
		store = NewContainerStore()
	})

	Describe("AddContainer", func() {
		BeforeEach(func() {
			store.AddContainer(id, ip, iamRole)
		})

		It("Adds a container to the store", func() {
			role, err := store.IAMRoleForIP(ip)
			Expect(role).To(Equal(iamRole))
			Expect(err).To(BeNil())
		})
	})

	Describe("RemoveContainer", func() {
		BeforeEach(func() {
			store.AddContainer(id, ip, iamRole)
			store.RemoveContainer(id)
		})

		It("Removes the container from the store", func() {
			role, err := store.IAMRoleForIP(ip)
			Expect(role).To(Equal(""))
			Expect(err).ToNot(BeNil())
		})
	})

	Describe("IAMRoleForIP", func() {
		Context("When the container is not in the store", func() {
			It("Returns an error", func() {
				role, err := store.IAMRoleForIP(ip)
				Expect(role).To(Equal(""))
				Expect(err).ToNot(BeNil())
			})
		})

		Context("When the container is in the store", func() {
			BeforeEach(func() {
				store.AddContainer(id, ip, iamRole)
			})

			It("Returns the role", func() {
				role, err := store.IAMRoleForIP(ip)
				Expect(role).To(Equal(iamRole))
				Expect(err).To(BeNil())
			})
		})
	})
})
