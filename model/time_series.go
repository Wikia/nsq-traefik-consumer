package model

import (
	"fmt"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
)

type TimeBucket struct {
	sync.RWMutex
	start time.Time // beginning of an epoch (inclusive)
	end   time.Time // end of an epoch (inclusive)
	items []TimePoint
}

func (tb *TimeBucket) IsEmpty() bool {
	return len(tb.items) == 0
}

func (tb *TimeBucket) Len() int {
	return len(tb.items)
}

func (tb *TimeBucket) Clone() *TimeBucket {
	res := TimeBucket{start: tb.start, end: tb.end}
	res.items = make([]TimePoint, len(tb.items))
	copy(res.items, tb.items)

	return &res
}

func (tb *TimeBucket) Append(pt TimePoint) error {
	if pt.Ts().Before(tb.start) || pt.Ts().After(tb.end) {
		log.WithFields(log.Fields{
			"start": tb.start,
			"end":   tb.end,
			"point": pt.Ts(),
		}).Error("Time point is out of boundaries")
		return fmt.Errorf("Time point is out of bucket boundaries")
	}

	tb.Lock()
	defer tb.Unlock()

	tb.items = append(tb.items, pt)
	return nil
}

func (tb *TimeBucket) Iter() func() (TimePoint, bool) {
	i := -1

	return func() (TimePoint, bool) {
		tb.Lock()
		defer tb.Unlock()
		i++
		if i == len(tb.items) {
			return nil, false
		}

		return tb.items[i], true
	}
}

type TimeSeries struct {
	sync.RWMutex
	start      time.Time
	resolution time.Duration
	data       []TimeBucket
}

func NewTimeSeries(resolution time.Duration) *TimeSeries {
	return &TimeSeries{resolution: resolution}
}

func (ts *TimeSeries) GetResolution() time.Duration {
	return ts.resolution
}

func (ts *TimeSeries) GetStart() time.Time {
	return ts.start
}

func (ts *TimeSeries) GetBucketCount() int {
	return len(ts.data)
}

func (ts *TimeSeries) calculateBucketIdx(val time.Time) int64 {
	return val.Sub(ts.start).Nanoseconds() / ts.resolution.Nanoseconds()
}

func (ts *TimeSeries) Clear() {
	ts.Lock()
	defer ts.Unlock()

	ts.data = []TimeBucket{}
}

func (ts *TimeSeries) Trim(start, end time.Time) uint64 {
	ts.Lock()
	defer ts.Unlock()

	bucketsDeleted := uint64(0)
	epochStart := start.Round(ts.resolution)
	epochEnd := end.Round(ts.resolution)

	startIdx := ts.calculateBucketIdx(epochStart)
	endIdx := ts.calculateBucketIdx(epochEnd)

	ts.start = epochStart
	ts.data = ts.data[startIdx:endIdx]

	return bucketsDeleted
}

func (ts *TimeSeries) alignTime(val time.Time) time.Time {
	aligned := val.Round(ts.resolution)
	if aligned.After(val) {
		aligned = aligned.Add(-ts.resolution)
	}

	return aligned
}

func (ts *TimeSeries) Append(tp TimePoint) error {
	ts.Lock()
	defer ts.Unlock()

	if ts.start.IsZero() {
		ts.start = ts.alignTime(tp.Ts())
	}

	bucketIdx := ts.calculateBucketIdx(tp.Ts())

	if bucketIdx >= int64(len(ts.data)) {
		newData := make([]TimeBucket, bucketIdx+1)
		copy(newData, ts.data)
		ts.data = newData
	}

	if ts.data[bucketIdx].IsEmpty() {
		epochStart := ts.alignTime(tp.Ts())
		ts.data[bucketIdx].start = epochStart
		ts.data[bucketIdx].end = epochStart.Add(ts.resolution)
	}

	return ts.data[bucketIdx].Append(tp)
}

func (ts *TimeSeries) Iter() func() (*TimeBucket, bool) {
	i := -1

	return func() (*TimeBucket, bool) {
		ts.Lock()
		defer ts.Unlock()
		i++
		if i == len(ts.data) {
			return nil, false
		}
		return &ts.data[i], true
	}
}

type PointGroup struct {
	Points      []TimePoint
	ExtraValues map[string]string
	Tags        map[string]string
}

type TimePoint interface {
	Ts() time.Time
	SetTs(time.Time)
	Clone() TimePoint
	String() string
}

type TimePointUInt64 struct {
	v  uint64
	ts time.Time
}

func NewTimePointUInt64(ts time.Time, val uint64) *TimePointUInt64 {
	return &TimePointUInt64{v: val, ts: ts}
}

func (tp *TimePointUInt64) Ts() time.Time       { return tp.ts }
func (tp *TimePointUInt64) SetTs(ts time.Time)  { tp.ts = ts }
func (tp *TimePointUInt64) Value() uint64       { return tp.v }
func (tp *TimePointUInt64) SetValue(val uint64) { tp.v = val }
func (tp *TimePointUInt64) String() string      { return fmt.Sprintf("%d", tp.v) }
func (tp TimePointUInt64) Clone() TimePoint     { return NewTimePointUInt64(tp.ts, tp.v) }

type TimePointFloat64 struct {
	v  float64
	ts time.Time
}

func NewTimePointFloat64(ts time.Time, val float64) *TimePointFloat64 {
	return &TimePointFloat64{v: val, ts: ts}
}

func (tp *TimePointFloat64) Ts() time.Time        { return tp.ts }
func (tp *TimePointFloat64) SetTs(ts time.Time)   { tp.ts = ts }
func (tp *TimePointFloat64) Value() float64       { return tp.v }
func (tp *TimePointFloat64) SetValue(val float64) { tp.v = val }
func (tp *TimePointFloat64) String() string       { return fmt.Sprintf("%g", tp.v) }
func (tp TimePointFloat64) Clone() TimePoint      { return NewTimePointFloat64(tp.ts, tp.v) }
