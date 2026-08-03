package cursor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	ociCommon "github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/common/auth"
	"github.com/oracle/oci-go-sdk/v65/objectstorage"
)

// Store persists per-log-group marks. The etag returned by Load must be passed
// back to Save so the write is conditional (optimistic concurrency).
type Store interface {
	// Load returns the mark for a log group, its etag, and whether it existed.
	Load(ctx context.Context, logGroupID string) (mark Mark, etag string, found bool, err error)
	// Save writes the mark. etag is the value from Load ("" when the object did
	// not exist), used for an If-Match / If-None-Match conditional PUT.
	Save(ctx context.Context, logGroupID string, mark Mark, etag string) error
}

// osClient is the subset of the Object Storage SDK client used here, extracted as
// an interface for testability.
type osClient interface {
	GetObject(ctx context.Context, request objectstorage.GetObjectRequest) (objectstorage.GetObjectResponse, error)
	PutObject(ctx context.Context, request objectstorage.PutObjectRequest) (objectstorage.PutObjectResponse, error)
}

// objectStore is the Object Storage-backed Store.
type objectStore struct {
	client    osClient
	namespace string
	bucket    string
	prefix    string
}

// NewObjectStore builds a Store backed by OCI Object Storage, authenticated via
// Resource Principal (matching the forwarder). region may be empty to use the
// Resource Principal's home region.
func NewObjectStore(namespace, bucket, prefix, region string) (Store, error) {
	provider, err := auth.ResourcePrincipalConfigurationProvider()
	if err != nil {
		return nil, fmt.Errorf("failed to create resource principal configuration provider: %w", err)
	}
	client, err := objectstorage.NewObjectStorageClientWithConfigurationProvider(provider)
	if err != nil {
		return nil, fmt.Errorf("failed to create object storage client: %w", err)
	}
	if region != "" {
		client.SetRegion(region)
	}
	return &objectStore{client: &client, namespace: namespace, bucket: bucket, prefix: prefix}, nil
}

func (s *objectStore) Load(ctx context.Context, lg string) (Mark, string, bool, error) {
	resp, err := s.client.GetObject(ctx, objectstorage.GetObjectRequest{
		NamespaceName: ociCommon.String(s.namespace),
		BucketName:    ociCommon.String(s.bucket),
		ObjectName:    ociCommon.String(objectName(s.prefix, lg)),
	})
	if err != nil {
		if isNotFound(err) {
			return Mark{}, "", false, nil
		}
		return Mark{}, "", false, fmt.Errorf("failed to get cursor object for %s: %w", lg, err)
	}
	defer resp.Content.Close()

	body, err := io.ReadAll(resp.Content)
	if err != nil {
		return Mark{}, "", false, fmt.Errorf("failed to read cursor object for %s: %w", lg, err)
	}
	var mark Mark
	if err := json.Unmarshal(body, &mark); err != nil {
		return Mark{}, "", false, fmt.Errorf("failed to unmarshal cursor object for %s: %w", lg, err)
	}
	etag := ""
	if resp.ETag != nil {
		etag = *resp.ETag
	}
	return mark, etag, true, nil
}

func (s *objectStore) Save(ctx context.Context, lg string, mark Mark, etag string) error {
	body, err := json.Marshal(mark)
	if err != nil {
		return fmt.Errorf("failed to marshal cursor for %s: %w", lg, err)
	}

	req := objectstorage.PutObjectRequest{
		NamespaceName: ociCommon.String(s.namespace),
		BucketName:    ociCommon.String(s.bucket),
		ObjectName:    ociCommon.String(objectName(s.prefix, lg)),
		ContentLength: ociCommon.Int64(int64(len(body))),
		PutObjectBody: io.NopCloser(bytes.NewReader(body)),
	}
	// Conditional write for optimistic concurrency: match the etag we read, or
	// require the object be absent if we never saw one.
	if etag != "" {
		req.IfMatch = ociCommon.String(etag)
	} else {
		req.IfNoneMatch = ociCommon.String("*")
	}

	if _, err := s.client.PutObject(ctx, req); err != nil {
		return fmt.Errorf("failed to put cursor object for %s: %w", lg, err)
	}
	return nil
}

// isNotFound reports whether an OCI SDK error is a 404.
func isNotFound(err error) bool {
	if svcErr, ok := ociCommon.IsServiceError(err); ok {
		return svcErr.GetHTTPStatusCode() == 404
	}
	return false
}

// IsPreconditionFailed reports whether an error is a 412 (etag mismatch), meaning
// another invocation wrote the cursor first. Callers treat this as a benign
// concurrent-update signal and fail open (a few duplicates beat data loss).
func IsPreconditionFailed(err error) bool {
	if svcErr, ok := ociCommon.IsServiceError(err); ok {
		return svcErr.GetHTTPStatusCode() == 412
	}
	return false
}
