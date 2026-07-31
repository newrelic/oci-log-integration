// Package cursor drops OCI log records already ingested during a Service
// Connector re-read window. After the reconciler modifies a connector, SCH
// re-delivers a rolling window of records; this keeps a per-log-group high-water
// timestamp in Object Storage and drops anything at or below it.
//
// POC: bare-minimum logic. Fails open (keeps records on any error), no
// concurrency handling, timestamp-only mark.
package cursor

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common/auth"
	"github.com/oracle/oci-go-sdk/v65/objectstorage"
)

type mark struct {
	LastTimeUnixNano int64 `json:"lastTimeUnixNano"`
}

// Apply returns only records newer than each log group's saved mark and advances
// the mark. Any error keeps the records untouched.
func Apply(ctx context.Context, namespace, bucket, prefix string, records []map[string]interface{}) []map[string]interface{} {
	client, err := newClient()
	if err != nil {
		return records
	}

	// newest timestamp per log group in this batch
	maxByGroup := map[string]int64{}
	for _, r := range records {
		lg := logGroupID(r)
		if lg == "" {
			continue
		}
		if t, ok := recordTime(r); ok && t > maxByGroup[lg] {
			maxByGroup[lg] = t
		}
	}

	// load each group's mark
	marks := map[string]int64{}
	for lg := range maxByGroup {
		marks[lg] = loadMark(ctx, client, namespace, bucket, prefix, lg)
	}

	// keep records strictly newer than the mark; keep anything we can't position
	out := make([]map[string]interface{}, 0, len(records))
	for _, r := range records {
		lg := logGroupID(r)
		t, ok := recordTime(r)
		if lg == "" || !ok || t > marks[lg] {
			out = append(out, r)
		}
	}

	// advance marks
	for lg, mx := range maxByGroup {
		if mx > marks[lg] {
			saveMark(ctx, client, namespace, bucket, prefix, lg, mx)
		}
	}
	return out
}

func newClient() (objectstorage.ObjectStorageClient, error) {
	provider, err := auth.ResourcePrincipalConfigurationProvider()
	if err != nil {
		return objectstorage.ObjectStorageClient{}, err
	}
	return objectstorage.NewObjectStorageClientWithConfigurationProvider(provider)
}

func objectName(prefix, logGroupID string) string {
	return prefix + "cursor/" + logGroupID + ".json"
}

func loadMark(ctx context.Context, c objectstorage.ObjectStorageClient, ns, bucket, prefix, lg string) int64 {
	name := objectName(prefix, lg)
	resp, err := c.GetObject(ctx, objectstorage.GetObjectRequest{
		NamespaceName: &ns, BucketName: &bucket, ObjectName: &name,
	})
	if err != nil {
		return 0 // missing or error -> treat as no mark
	}
	defer resp.Content.Close()
	b, err := io.ReadAll(resp.Content)
	if err != nil {
		return 0
	}
	var m mark
	if json.Unmarshal(b, &m) != nil {
		return 0
	}
	return m.LastTimeUnixNano
}

func saveMark(ctx context.Context, c objectstorage.ObjectStorageClient, ns, bucket, prefix, lg string, t int64) {
	name := objectName(prefix, lg)
	body, _ := json.Marshal(mark{LastTimeUnixNano: t})
	length := int64(len(body))
	_, _ = c.PutObject(ctx, objectstorage.PutObjectRequest{
		NamespaceName: &ns, BucketName: &bucket, ObjectName: &name,
		ContentLength: &length, PutObjectBody: io.NopCloser(bytes.NewReader(body)),
	})
}

func recordTime(r map[string]interface{}) (int64, bool) {
	if v, ok := r["time"].(string); ok && v != "" {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			return t.UnixNano(), true
		}
	}
	return 0, false
}

func logGroupID(r map[string]interface{}) string {
	if o, ok := r["oracle"].(map[string]interface{}); ok {
		if v, ok := o["loggroupid"].(string); ok {
			return v
		}
	}
	return ""
}
