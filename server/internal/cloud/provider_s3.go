package cloud

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// S3Provider is a placeholder CloudProvider for AWS S3 / Cloudflare R2.
//
// TODO: implement using github.com/aws/aws-sdk-go-v2/service/s3
//   - List: s3.ListObjectsV2 with ContinuationToken
//   - Download: s3manager.Downloader
type S3Provider struct {
	bucket string
	prefix string // optional key prefix under the bucket
}

// NewS3Provider creates a placeholder S3 provider.
func NewS3Provider(bucket, prefix string) *S3Provider {
	return &S3Provider{bucket: bucket, prefix: prefix}
}

func (p *S3Provider) Name() ProviderKind { return ProviderS3 }

func (p *S3Provider) List(ctx context.Context, repoID uuid.UUID, cursor *Cursor) (*Page, error) {
	return nil, fmt.Errorf("s3 provider not implemented")
}

func (p *S3Provider) Download(ctx context.Context, repoID uuid.UUID, remoteKey string, localPath string) (int64, error) {
	return 0, fmt.Errorf("s3 provider not implemented")
}
