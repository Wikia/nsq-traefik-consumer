package model_test

import (
	. "github.com/Wikia/nsq-traefik-consumer/model"

	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TimeSeries", func() {
	ts := NewTimeSeries(10 * time.Second)
	BeforeEach(func() {
		ts.Clear()
	})

	It("Should be able to handle multiple TimePointUInt64", func() {
		startTime := time.Now()
		pt := NewTimePointUInt64(startTime, 1)

		Expect(ts.GetBucketCount()).To(Equal(0), "Check if there are no buckets in TimeSeries")

		err := ts.Append(&pt)

		Expect(err).NotTo(HaveOccurred(), "Successfully added TimePoint")
		Expect(ts.GetBucketCount()).To(Equal(1), "Bucket count should increase by 1")
	})
})
