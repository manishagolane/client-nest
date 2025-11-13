package clients

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"time"

	"github.com/manishagolane/client-nest/config"
	"github.com/manishagolane/client-nest/constants"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type StorageClient struct {
	s3Client      *s3.Client
	presignClient *s3.PresignClient
}

func NewStorageClient(region string) (*StorageClient, error) {
	// Load IAM role credentials using the default AWS SDK config.
	s3cfg, err := awsConfig.LoadDefaultConfig(context.TODO(), awsConfig.WithRegion(region))
	if err != nil {
		log.Fatal("error loading AWS SDK config", err)
	}
	// Create an S3 client using the IAM role credentials.
	s3Client := s3.NewFromConfig(s3cfg)

	// Create S3 Presign Client
	presignClient := s3.NewPresignClient(s3Client)

	return &StorageClient{
		s3Client:      s3Client,
		presignClient: presignClient,
	}, nil
}

func (s StorageClient) UploadFile(ctx context.Context, fileData []byte, fileName string, contentType string) (string, error) {
	bucketName := config.GetString("aws.s3Bucket")
	_, err := s.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucketName),
		Key:         aws.String(fileName),
		Body:        bytes.NewReader(fileData),
		ContentType: aws.String(contentType),
	})

	if err != nil {
		return "", fmt.Errorf("failed to upload file to S3: %v", err)
	}

	fileURL := fmt.Sprintf("https://%s.s3.amazonaws.com/%s", bucketName, fileName)
	// log.Printf("File uploaded successfully: %s", fileURL)
	return fileURL, nil
}

func (sc StorageClient) ReadObject(ctx context.Context, bucket, key string) (*s3.GetObjectOutput, error) {
	response, err := sc.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Key:    &key,
		Bucket: &bucket,
	})
	if err != nil {
		log.Println("Error storing object", err)
		return nil, err
	}
	return response, nil
}

func (sc StorageClient) DeleteObject(ctx context.Context, bucket, key string) error {
	_, err := sc.s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		log.Println("Error deleting object", err)
		return err
	}
	log.Println("Deleted object from S3", key)
	return nil
}

func (sc *StorageClient) GeneratePresignedURL(ctx context.Context, s3Key, method string, expiration time.Duration) (string, error) {
	bucketName := config.GetString("aws.s3Bucket")
	input := &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(s3Key),
	}

	presignedReq, err := sc.presignClient.PresignPutObject(ctx, input, func(po *s3.PresignOptions) {
		po.Expires = expiration
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate pre-signed URL: %v", err)
	}

	// Append maxSize to the URL as a query param
	presignedURL := fmt.Sprintf("%s&maxSize=%d", presignedReq.URL, constants.MaxFileSize)

	return presignedURL, nil
}
