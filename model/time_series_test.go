package model_test

import (
	. "github.com/Wikia/nsq-traefik-consumer/model"

	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TimeSeries", func() {
	ts := NewTimeSeries(10 * time.Second)
	// we should normalize time to be able to predict the buckets they will fall into
	startTime := time.Now().Round(10 * time.Second).Add(time.Second)

	BeforeEach(func() {
		ts.Clear()
		Expect(ts.GetBucketCount()).To(Equal(0), "Check if there are no buckets in TimeSeries")
	})

	It("Should be able to handle single TimePointUInt64", func() {
		pt := NewTimePointUInt64(startTime, 1)

		err := ts.Append(pt)
		Expect(err).NotTo(HaveOccurred(), "Successfully added TimePoint")
		Expect(ts.GetBucketCount()).To(Equal(1), "Bucket count should increase by 1")

		bucketIter := ts.Iter()
		for bucket, ok := bucketIter(); ok; bucket, ok = bucketIter() {
			Expect(bucket.IsEmpty()).To(BeFalse())
			Expect(bucket.Len()).To(Equal(1))

			timePointIter := bucket.Iter()
			for timePoint, ok := timePointIter(); ok; timePoint, ok = timePointIter() {
				Expect(timePoint).To(BeEquivalentTo(pt))
			}
		}
	})

	It("Should properly separate TimePoints into buckets", func() {
		pt1 := NewTimePointUInt64(startTime, 1)
		pt2 := NewTimePointUInt64(startTime.Add(5*time.Second), 2)
		pt3 := NewTimePointUInt64(startTime.Add(10*time.Second), 3)
		pt4 := NewTimePointUInt64(startTime.Add(15*time.Second), 4)

		err := ts.Append(pt1)
		Expect(err).NotTo(HaveOccurred(), "Successfully added #1 TimePoint")
		err = ts.Append(pt2)
		Expect(err).NotTo(HaveOccurred(), "Successfully added #2 TimePoint")

		Expect(ts.GetBucketCount()).To(Equal(1), "There should be exactly 1 bucket")

		err = ts.Append(pt3)
		Expect(err).NotTo(HaveOccurred(), "Successfully added #3 TimePoint")
		err = ts.Append(pt4)
		Expect(err).NotTo(HaveOccurred(), "Successfully added #4 TimePoint")

		Expect(ts.GetBucketCount()).To(Equal(2), "There should be exactly 2 buckets")

		pt5 := NewTimePointUInt64(startTime.Add(21*time.Second), 5)
		err = ts.Append(pt5)
		Expect(err).NotTo(HaveOccurred(), "Successfully added #5 TimePoint")

		Expect(ts.GetBucketCount()).To(Equal(3), "There should be exactly 3 buckets")

		pt6 := NewTimePointUInt64(startTime.Add(101*time.Second), 6)
		err = ts.Append(pt6)
		Expect(err).NotTo(HaveOccurred(), "Successfully added #6 TimePoint")

		Expect(ts.GetBucketCount()).To(Equal(11), "There should be exactly 11 buckets")

	})
})
