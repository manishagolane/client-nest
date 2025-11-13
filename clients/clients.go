package clients

import (
	"github.com/manishagolane/client-nest/cache"
	"github.com/manishagolane/client-nest/config"
	"github.com/manishagolane/client-nest/models"

	"go.uber.org/zap"
)

type Clients struct {
	EmailClient *EmailClient
	S3Client    *StorageClient
	NATSClient  *NATSClient
	// OTPClient   *OTPClient
}

func InitializeClients(logger *zap.Logger, cache *cache.Cache, queries *models.Queries) *Clients {
	logger.Info("Initializing email Client")
	emailClient, err := NewEmailClient()
	if err != nil {
		logger.Fatal("Failed to initialize EmailClient", zap.Error(err))
	}

	region := config.GetString("aws.region")

	s3Client, err := NewStorageClient(region)
	if err != nil {
		logger.Fatal("Failed to initialize S3Client", zap.Error(err))
	}

	natsClient, err := NewNATSClient(logger)
	if err != nil {
		logger.Fatal("Failed to initialize NATSClient", zap.Error(err))
	}

	// OTPClient, err := NewOTPgRPCClient(logger, cache)
	// if err != nil {
	// 	logger.Fatal("Failed to initialize otpService", zap.Error(err))
	// }

	client := &Clients{
		EmailClient: emailClient,
		S3Client:    s3Client,
		NATSClient:  natsClient,
		// OTPClient:   OTPClient,
	}

	return client
}
