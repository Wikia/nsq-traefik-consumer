package common_test

import (
	"time"

	. "github.com/Wikia/continuous-deployment-poc/common"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Version", func() {
	Describe("GetCurrentVersion()", func() {
		Context("With date '1985-04-12T23:20:50.52Z' and version 1.0.0", func() {
			BeforeEach(func() {
				Version = "1.0.0"
				date, _ := time.Parse(time.RFC3339Nano, "1985-04-12T23:20:50.52Z")
				BuildTime = date.Format(time.RFC3339Nano)
			})

			It("should return proper values", func() {
				Expect(GetCurrentVersion().BuildTime).To(Equal("1985-04-12T23:20:50.52Z"))
				Expect(GetCurrentVersion().Version).To(Equal("1.0.0"))
			})

			It("should properly format build time and version", func() {
				Expect(GetCurrentVersion().String()).To(Equal("1.0.0 (time: 1985-04-12T23:20:50.52Z)"))
			})
		})

		Context("With date empty and version 1.0.0", func() {
			BeforeEach(func() {
				Version = "1.0.0"
				BuildTime = ""
			})

			It("should return proper values", func() {
				Expect(GetCurrentVersion().BuildTime).To(Equal(""))
				Expect(GetCurrentVersion().Version).To(Equal("1.0.0"))
			})

			It("should properly format version", func() {
				Expect(GetCurrentVersion().String()).To(Equal("1.0.0"))
			})
		})
	})
})
