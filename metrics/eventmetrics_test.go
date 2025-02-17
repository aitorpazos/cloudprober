// Copyright 2017 The Cloudprober Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metrics

import (
	"fmt"
	"testing"
	"time"
)

func newEventMetrics(sent, rcvd, rtt int64, respCodes map[string]int64) *EventMetrics {
	respCodesVal := NewMap("code")
	for k, v := range respCodes {
		respCodesVal.IncKeyBy(k, v)
	}
	em := NewEventMetrics(time.Now()).
		AddMetric("sent", NewInt(sent)).
		AddMetric("rcvd", NewInt(rcvd)).
		AddMetric("rtt", NewInt(rtt)).
		AddMetric("resp-code", respCodesVal)
	return em
}

func verifyOrder(em *EventMetrics, names ...string) error {
	keys := em.MetricsKeys()
	for i := range names {
		if keys[i] != names[i] {
			return fmt.Errorf("Metrics not in order. At Index: %d, Expected: %s, Got: %s", i, names[i], keys[i])
		}
	}
	return nil
}

func verifyEventMetrics(t *testing.T, m *EventMetrics, sent, rcvd, rtt int64, respCodes map[string]int64) {
	// Verify that metrics are ordered correctly.
	if err := verifyOrder(m, "sent", "rcvd", "rtt", "resp-code"); err != nil {
		t.Error(err)
	}

	expectedMetrics := map[string]int64{
		"sent": sent,
		"rcvd": rcvd,
		"rtt":  rtt,
	}
	for k, eVal := range expectedMetrics {
		if m.Metric(k).(NumValue).Int64() != eVal {
			t.Errorf("Unexpected metric value. Expected: %d, Got: %d", eVal, m.Metric(k).(*Int).Int64())
		}
	}
	for k, eVal := range respCodes {
		if m.Metric("resp-code").(*Map[int64]).GetKey(k) != eVal {
			t.Errorf("Unexpected metric value. Expected: %d, Got: %d", eVal, m.Metric("resp-code").(*Map[int64]).GetKey(k))
		}
	}
}

func TestEventMetricsUpdate(t *testing.T) {
	m := newEventMetrics(0, 0, 0, make(map[string]int64))
	m.AddLabel("ptype", "http")

	m2 := newEventMetrics(32, 22, 220100, map[string]int64{
		"200": 22,
	})
	m.Update(m2)
	// We'll verify later that mClone is un-impacted by further updates.
	mClone := m.Clone()

	// Verify that "m" has been updated correctly.
	verifyEventMetrics(t, m, 32, 22, 220100, map[string]int64{
		"200": 22,
	})

	m3 := newEventMetrics(30, 30, 300100, map[string]int64{
		"200": 22,
		"204": 8,
	})
	m.Update(m3)

	// Verify that "m" has been updated correctly.
	verifyEventMetrics(t, m, 62, 52, 520200, map[string]int64{
		"200": 44,
		"204": 8,
	})

	// Verify that even though "m" has changed, mClone is as m was after first update
	verifyEventMetrics(t, mClone, 32, 22, 220100, map[string]int64{
		"200": 22,
	})

	// Log metrics in string format
	// t.Log(m.String())

	expectedString := fmt.Sprintf("%d labels=ptype=http sent=62 rcvd=52 rtt=520200 resp-code=map:code,200:44,204:8", m.Timestamp.Unix())
	s := m.String()
	if s != expectedString {
		t.Errorf("em.String()=%s, want=%s", s, expectedString)
	}
}

func TestEventMetricsSubtractCounters(t *testing.T) {
	m := newEventMetrics(10, 10, 1000, make(map[string]int64))
	m.AddLabel("ptype", "http")

	// First run
	m2 := newEventMetrics(32, 22, 220100, map[string]int64{
		"200": 22,
	})
	gEM, err := m2.SubtractLast(m)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	verifyEventMetrics(t, gEM, 22, 12, 219100, map[string]int64{
		"200": 22,
	})

	// Second run
	m3 := newEventMetrics(42, 31, 300100, map[string]int64{
		"200": 24,
		"204": 8,
	})

	gEM, err = m3.SubtractLast(m2)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	verifyEventMetrics(t, gEM, 10, 9, 80000, map[string]int64{
		"200": 2,
		"204": 8,
	})

	// Third run, expect reset
	m4 := newEventMetrics(10, 8, 1100, map[string]int64{
		"200": 8,
	})
	gEM, err = m4.SubtractLast(m3)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	verifyEventMetrics(t, gEM, 10, 8, 1100, map[string]int64{
		"200": 8,
	})
}

func TestKey(t *testing.T) {
	m := newEventMetrics(42, 31, 300100, map[string]int64{
		"200": 24,
		"204": 8,
	}).AddLabel("probe", "google-homepage")

	key := m.Key()
	wantKey := "sent,rcvd,rtt,resp-code,probe=google-homepage"

	if key != wantKey {
		t.Errorf("Got key: %s, wanted: %s", key, wantKey)
	}
}

func BenchmarkEventMetricsStringer(b *testing.B) {
	em := newEventMetrics(32, 22, 220100, map[string]int64{
		"200": 22,
		"404": 4500,
		"403": 4500,
	})
	// run the em.String() function b.N times
	for n := 0; n < b.N; n++ {
		_ = em.String()
	}
}

func TestAllocsPerRun(t *testing.T) {
	respCodesVal := NewMap("code")
	for k, v := range map[string]int64{
		"200": 22,
		"404": 4500,
		"403": 4500,
	} {
		respCodesVal.IncKeyBy(k, v)
	}

	var em *EventMetrics
	newAvg := testing.AllocsPerRun(100, func() {
		em = NewEventMetrics(time.Now()).
			AddMetric("sent", NewInt(32)).
			AddMetric("rcvd", NewInt(22)).
			AddMetric("rtt", NewInt(220100)).
			AddMetric("resp-code", respCodesVal)
	})

	stringAvg := testing.AllocsPerRun(100, func() {
		_ = em.String()
	})

	t.Logf("Average allocations per run: ForNew=%v, ForString=%v", newAvg, stringAvg)
}
