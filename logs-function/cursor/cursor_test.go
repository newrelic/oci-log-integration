package cursor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func nano(ts string) int64 {
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		panic(err)
	}
	return t.UnixNano()
}

func rec(idx int, id, ts string) record {
	tn, ok := parseRecordTime(map[string]interface{}{"time": ts})
	return record{id: id, timeNano: tn, hasTime: ok, idx: idx}
}

func TestFilter_EmptyMarkKeepsAll(t *testing.T) {
	recs := []record{
		rec(0, "a", "2026-07-31T10:00:00Z"),
		rec(1, "b", "2026-07-31T10:00:01Z"),
	}
	kept, updated, dropped := filter(recs, Mark{})

	assert.Len(t, kept, 2)
	assert.Equal(t, 0, dropped)
	assert.Equal(t, nano("2026-07-31T10:00:01Z"), updated.LastTimeUnixNano)
	assert.Equal(t, []string{"b"}, updated.SeenIDs)
}

func TestFilter_DropsOlderThanMark(t *testing.T) {
	mark := Mark{LastTimeUnixNano: nano("2026-07-31T10:00:05Z")}
	recs := []record{
		rec(0, "old", "2026-07-31T10:00:01Z"),
		rec(1, "new", "2026-07-31T10:00:09Z"),
	}
	kept, updated, dropped := filter(recs, mark)

	require.Len(t, kept, 1)
	assert.Equal(t, "new", kept[0].id)
	assert.Equal(t, 1, dropped)
	assert.Equal(t, nano("2026-07-31T10:00:09Z"), updated.LastTimeUnixNano)
}

// Records sharing the boundary timestamp: already-seen ids are dropped, new ones
// at the same instant are kept.
func TestFilter_BoundaryTiesUseSeenIDs(t *testing.T) {
	boundary := "2026-07-31T10:00:05Z"
	mark := Mark{LastTimeUnixNano: nano(boundary), SeenIDs: []string{"a"}}
	recs := []record{
		rec(0, "a", boundary), // already seen -> drop
		rec(1, "b", boundary), // same instant, new -> keep
	}
	kept, updated, dropped := filter(recs, mark)

	require.Len(t, kept, 1)
	assert.Equal(t, "b", kept[0].id)
	assert.Equal(t, 1, dropped)
	assert.Equal(t, nano(boundary), updated.LastTimeUnixNano)
	assert.ElementsMatch(t, []string{"a", "b"}, updated.SeenIDs, "boundary ids accumulate when max does not advance")
}

func TestFilter_UnparseableTimeKept(t *testing.T) {
	mark := Mark{LastTimeUnixNano: nano("2026-07-31T10:00:05Z")}
	recs := []record{{id: "x", hasTime: false, idx: 0}}
	kept, _, dropped := filter(recs, mark)

	assert.Len(t, kept, 1)
	assert.Equal(t, 0, dropped)
}

func TestParseRecordTime(t *testing.T) {
	tn, ok := parseRecordTime(map[string]interface{}{"time": "2026-07-31T10:00:00Z"})
	assert.True(t, ok)
	assert.Equal(t, nano("2026-07-31T10:00:00Z"), tn)

	tn, ok = parseRecordTime(map[string]interface{}{"datetime": float64(1000)})
	assert.True(t, ok)
	assert.Equal(t, int64(1000)*int64(time.Millisecond), tn)

	_, ok = parseRecordTime(map[string]interface{}{"foo": "bar"})
	assert.False(t, ok)
}

func TestExtractors(t *testing.T) {
	raw := map[string]interface{}{
		"id":     "rec-1",
		"oracle": map[string]interface{}{"loggroupid": "ocid1.loggroup.oc1..x"},
	}
	assert.Equal(t, "rec-1", recordID(raw))
	assert.Equal(t, "ocid1.loggroup.oc1..x", logGroupID(raw))
	assert.Equal(t, "p/cursor/ocid1.loggroup.oc1..x.json", objectName("p/", "ocid1.loggroup.oc1..x"))
}
