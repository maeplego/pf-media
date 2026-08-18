package objectstore

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Client struct {
	mc        *minio.Client
	presignMC *minio.Client
	bucket    string
}

func New(endpoint, publicEndpoint, region, accessKey, secretKey, bucket string, useSSL bool) (*Client, error) {
	opts := func(host string) (*minio.Client, error) {
		return minio.New(host, &minio.Options{
			Creds:        credentials.NewStaticV4(accessKey, secretKey, ""),
			Secure:       useSSL,
			Region:       region,
			BucketLookup: minio.BucketLookupPath,
		})
	}
	mc, err := opts(endpoint)
	if err != nil {
		return nil, err
	}
	presignEndpoint := publicEndpoint
	if presignEndpoint == "" {
		presignEndpoint = endpoint
	}
	presignMC := mc
	if presignEndpoint != endpoint {
		presignMC, err = opts(presignEndpoint)
		if err != nil {
			return nil, err
		}
	}
	return &Client{mc: mc, presignMC: presignMC, bucket: bucket}, nil
}

func (c *Client) EnsureBucket(ctx context.Context) error {
	ok, err := c.mc.BucketExists(ctx, c.bucket)
	if err != nil {
		return err
	}
	if !ok {
		return c.mc.MakeBucket(ctx, c.bucket, minio.MakeBucketOptions{})
	}
	return nil
}

func (c *Client) PresignPut(ctx context.Context, key, contentType string, ttl time.Duration) (*url.URL, error) {
	return c.presignMC.PresignedPutObject(ctx, c.bucket, key, ttl)
}

func (c *Client) PresignGet(ctx context.Context, key string, ttl time.Duration) (*url.URL, error) {
	return c.presignMC.PresignedGetObject(ctx, c.bucket, key, ttl, nil)
}

func (c *Client) Stat(ctx context.Context, key string) (int64, string, error) {
	info, err := c.mc.StatObject(ctx, c.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		return 0, "", err
	}
	return info.Size, info.ETag, nil
}

func (c *Client) Delete(ctx context.Context, key string) error {
	return c.mc.RemoveObject(ctx, c.bucket, key, minio.RemoveObjectOptions{})
}

// DeletePrefix removes orig and derived objects under a file prefix (must be non-empty).
func (c *Client) DeletePrefix(ctx context.Context, prefix string) error {
	if prefix == "" || prefix == "/" {
		return fmt.Errorf("refusing empty object prefix")
	}
	listed := c.mc.ListObjects(ctx, c.bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})
	for err := range c.mc.RemoveObjects(ctx, c.bucket, listed, minio.RemoveObjectsOptions{}) {
		if err.Err != nil {
			return err.Err
		}
	}
	return nil
}

func (c *Client) Bucket() string { return c.bucket }

func ObjectKey(ownerSub, fileID string) string {
	return fmt.Sprintf("user/%s/%s/orig", ownerSub, fileID)
}

func ObjectPrefix(ownerSub, fileID string) string {
	return fmt.Sprintf("user/%s/%s/", ownerSub, fileID)
}

func VariantKey(ownerSub, fileID, variant string) string {
	return fmt.Sprintf("user/%s/%s/%s", ownerSub, fileID, variant)
}
