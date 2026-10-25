package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// remoteObject is one object in the remote mirror listing.
type remoteObject struct {
	Key          string    `json:"key"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"lastModified"`
}

// s3ClientFor builds a generic S3-compatible client for a destination. Every
// S3-compatible provider (AWS, Cloudflare R2, MinIO, Wasabi, B2, Hetzner,
// iDrive...) shares this shape: custom endpoint + credentials + optional
// path-style addressing — the same information model Dokploy feeds rclone,
// implemented natively in Go instead of shelling out to the rclone binary.
func s3ClientFor(ctx context.Context, d BackupDestination) (*s3.Client, error) {
	if d.Endpoint == "" || d.Bucket == "" || d.AccessKey == "" || d.SecretKey == "" {
		return nil, errors.New("destination requires endpoint, bucket, accessKey and secretKey")
	}
	region := d.Region
	if region == "" {
		region = "us-east-1"
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(d.AccessKey, d.SecretKey, ""),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("aws config: %w", err)
	}
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(strings.TrimSpace(d.Endpoint))
		// Path-style is the compatibility mechanism for non-AWS providers:
		// https://host/bucket/object instead of bucket.host/object virtual
		// hosting, which breaks on custom endpoints.
		o.UsePathStyle = d.PathStyle
	})
	return client, nil
}

// uploadToDestination streams a local snapshot file to the destination bucket
// and returns the size plus the sha256 checksum of the file contents.
// Multipart (8 MiB parts) via the SDK transfer manager keeps memory flat on
// small hosts; the checksum pass re-reads the local file afterwards.
func uploadToDestination(ctx context.Context, d BackupDestination, localPath, remoteKey string) (int64, string, error) {
	client, err := s3ClientFor(ctx, d)
	if err != nil {
		return 0, "", err
	}
	f, err := os.Open(localPath)
	if err != nil {
		return 0, "", fmt.Errorf("open snapshot for upload: %w", err)
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return 0, "", fmt.Errorf("stat snapshot: %w", err)
	}

	uploader := manager.NewUploader(client, func(u *manager.Uploader) {
		u.Concurrency = 3
		u.PartSize = 8 << 20
	})
	if _, err := uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket: aws.String(d.Bucket),
		Key:    aws.String(remoteKey),
		Body:   f,
	}); err != nil {
		return 0, "", fmt.Errorf("s3 upload: %w", err)
	}

	// Independent checksum pass over the local file — the same bytes that
	// were streamed (concurrent modification is a non-factor during the
	// quiesced export window).
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return fi.Size(), "", fmt.Errorf("rewind snapshot for checksum: %w", err)
	}
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fi.Size(), "", fmt.Errorf("checksum pass: %w", err)
	}
	return fi.Size(), hex.EncodeToString(h.Sum(nil)), nil
}

// deleteFromDestination removes the mirrored object best-effort; a missing
// object is not an error (the mirror may have been pruned already).
func deleteFromDestination(ctx context.Context, d BackupDestination, remoteKey string) error {
	client, err := s3ClientFor(ctx, d)
	if err != nil {
		return err
	}
	_, err = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(d.Bucket),
		Key:    aws.String(remoteKey),
	})
	if err != nil {
		if strings.Contains(err.Error(), "NoSuchKey") || strings.Contains(err.Error(), "NotFound") {
			return nil
		}
		return fmt.Errorf("s3 delete: %w", err)
	}
	return nil
}

// testDestination pings the bucket with the cheapest read available across
// providers (ListObjectsV2 with MaxKeys=1), under a strict timeout so a
// misconfigured endpoint cannot hang the settings page.
func testDestination(ctx context.Context, d BackupDestination) error {
	client, err := s3ClientFor(ctx, d)
	if err != nil {
		return err
	}
	tctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err = client.ListObjectsV2(tctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(d.Bucket),
		MaxKeys: aws.Int32(1),
	})
	if err != nil {
		return fmt.Errorf("s3 reachable check: %w", err)
	}
	return nil
}

// listRemote lists mirrored snapshot objects for a snapshot-history view.
func listRemote(ctx context.Context, d BackupDestination, prefix string, maxKeys int32) ([]remoteObject, error) {
	client, err := s3ClientFor(ctx, d)
	if err != nil {
		return nil, err
	}
	if maxKeys <= 0 {
		maxKeys = 100
	}
	prefix = strings.Trim(prefix, "/")
	if prefix != "" {
		prefix += "/"
	}
	out, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(d.Bucket),
		Prefix:  aws.String(prefix),
		MaxKeys: aws.Int32(maxKeys),
	})
	if err != nil {
		return nil, fmt.Errorf("s3 list: %w", err)
	}
	objects := make([]remoteObject, 0, len(out.Contents))
	for _, c := range out.Contents {
		objects = append(objects, remoteObject{
			Key:          aws.ToString(c.Key),
			Size:         aws.ToInt64(c.Size),
			LastModified: aws.ToTime(c.LastModified),
		})
	}
	return objects, nil
}

// remoteKeyFor builds the object key under the destination's optional prefix:
// <prefix>/<volume>/<filename>.
func remoteKeyFor(d BackupDestination, snap VolumeSnapshot) string {
	prefix := strings.Trim(d.Prefix, "/")
	parts := []string{}
	if prefix != "" {
		parts = append(parts, prefix)
	}
	parts = append(parts, snap.VolumeName, snap.Filename)
	return strings.Join(parts, "/")
}
