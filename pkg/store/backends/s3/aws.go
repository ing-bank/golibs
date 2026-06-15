package s3

import (
	"bytes"
	"context"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// ClientAWS implements Client using the official AWS SDK.
type ClientAWS struct {
	Client *s3.Client
}

// ConfigAWS holds S3 connection configuration.
type ConfigAWS struct {
	Namespace string
	Username  string
	Password  string
	URL       string
}

// NewAwsS3Client initializes an ClientAWS using the provided config.
func NewAwsS3Client(cfg ConfigAWS) (*ClientAWS, error) {
	awsCfg, err := awsConfigFromCustom(cfg)
	if err != nil {
		return nil, err
	}
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})
	return &ClientAWS{Client: client}, nil
}

// awsConfigFromCustom builds an AWS config from custom S3 credentials and endpoint.
func awsConfigFromCustom(cfg ConfigAWS) (aws.Config, error) {
	resolver := aws.EndpointResolverFunc(
		func(service, region string) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:           cfg.URL,
				SigningRegion: cfg.Namespace,
			}, nil
		},
	)
	return aws.Config{
		Region: cfg.Namespace,
		Credentials: aws.NewCredentialsCache(
			credentials.NewStaticCredentialsProvider(cfg.Username, cfg.Password, ""),
		),
		EndpointResolver: resolver,
	}, nil
}

func (a *ClientAWS) PutObject(ctx context.Context, bucket, key string, body []byte) error {
	_, err := a.Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(body),
	})
	return err
}

func (a *ClientAWS) GetObject(ctx context.Context, bucket, key string) ([]byte, error) {
	out, err := a.Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	defer out.Body.Close()
	return io.ReadAll(out.Body)
}

func (a *ClientAWS) DeleteObject(ctx context.Context, bucket, key string) error {
	_, err := a.Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	return err
}

func (a *ClientAWS) ListObjects(ctx context.Context, bucket string) ([]string, error) {
	var keys []string
	paginator := s3.NewListObjectsV2Paginator(a.Client, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, obj := range page.Contents {
			keys = append(keys, *obj.Key)
		}
	}
	return keys, nil
}

func (a *ClientAWS) CreateBucket(ctx context.Context, bucket string) error {
	_, err := a.Client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: &bucket,
	})
	return err
}

func (a *ClientAWS) DeleteBucket(ctx context.Context, bucket string) error {
	_, err := a.Client.DeleteBucket(ctx, &s3.DeleteBucketInput{
		Bucket: &bucket,
	})
	return err
}
