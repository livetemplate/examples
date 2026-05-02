package main

import (
	"bytes"
	"io"
	"testing"

	"github.com/livetemplate/livetemplate"
)

// BenchmarkLargeTable_UpdateRandomRow_WireBytes closes Open Question 2 in
// the streaming-range proposal: at N=10k, a single-field whole-item update
// must stay below the wire-cost ceiling that would justify a future
// targeted-field op (`["uf", key, fieldIdx, value]`). Project policy
// (per the user-approved Phase 6 plan): ceiling is 30% of full-tree size.
//
// The benchmark renders once (Execute → first render with statics), then
// calls ExecuteUpdates twice — first to transition to stream mode, then
// repeatedly to measure the per-update wire cost reported via b.ReportMetric.
//
// Each iteration mutates ONE field on ONE row; the wire output should be
// dominated by a single ["u", key, dynamics] op carrying all five fields
// of that row. With no sort active, no reorder op is emitted.
func BenchmarkLargeTable_UpdateRandomRow_WireBytes(b *testing.B) {
	cases := []struct {
		name string
		n    int
	}{
		{"N=200", 200},
		{"N=1000", 1000},
		{"N=10000", 10000},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			tmpl := livetemplate.Must(livetemplate.New("layout",
				livetemplate.WithParseFiles(
					"templates/layout.tmpl",
					"templates/lists/large-table.tmpl",
				),
			))

			c := newLargeTableController()
			c.rows = largeTableSeed(tc.n)
			c.nextID = tc.n + 1
			c.seedSize = tc.n

			state := c.refreshView(LargeTableState{
				Title:    "Large Table",
				Category: "Lists & Data",
			})

			if err := tmpl.Execute(io.Discard, state); err != nil {
				b.Fatalf("initial Execute: %v", err)
			}
			if err := tmpl.ExecuteUpdates(io.Discard, state); err != nil {
				b.Fatalf("transition ExecuteUpdates: %v", err)
			}

			var buf bytes.Buffer
			b.ResetTimer()
			b.ReportAllocs()

			var totalBytes int64
			for i := 0; i < b.N; i++ {
				idx := i % tc.n
				c.mu.Lock()
				c.rows[idx].Score = (c.rows[idx].Score + 1) % 1000
				c.mu.Unlock()
				state = c.refreshView(state)

				buf.Reset()
				if err := tmpl.ExecuteUpdates(&buf, state); err != nil {
					b.Fatalf("ExecuteUpdates iter %d: %v", i, err)
				}
				totalBytes += int64(buf.Len())
			}
			b.ReportMetric(float64(totalBytes)/float64(b.N), "wire-B/op")
		})
	}
}

// BenchmarkLargeTable_FullTreeBaseline_WireBytes is the legacy comparison
// point — render the FIRST tree (which always carries statics + every
// dynamic) and report its byte size. This is the size every subsequent
// render would carry if streaming-range diff did NOT exist (or fell back
// to full-tree replacement). Used for OQ2's "% of full-tree" ratio.
func BenchmarkLargeTable_FullTreeBaseline_WireBytes(b *testing.B) {
	cases := []struct {
		name string
		n    int
	}{
		{"N=200", 200},
		{"N=1000", 1000},
		{"N=10000", 10000},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			tmpl := livetemplate.Must(livetemplate.New("layout",
				livetemplate.WithParseFiles(
					"templates/layout.tmpl",
					"templates/lists/large-table.tmpl",
				),
			))

			c := newLargeTableController()
			c.rows = largeTableSeed(tc.n)
			c.nextID = tc.n + 1
			c.seedSize = tc.n

			state := c.refreshView(LargeTableState{
				Title:    "Large Table",
				Category: "Lists & Data",
			})

			var buf bytes.Buffer
			b.ResetTimer()
			b.ReportAllocs()

			var totalBytes int64
			for i := 0; i < b.N; i++ {
				buf.Reset()
				if err := tmpl.Execute(&buf, state); err != nil {
					b.Fatalf("Execute iter %d: %v", i, err)
				}
				totalBytes += int64(buf.Len())
			}
			b.ReportMetric(float64(totalBytes)/float64(b.N), "wire-B/op")
		})
	}
}
