package clients

import (
	"context"
	"fmt"
	"log"

	"github.com/manishagolane/client-nest/config"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
)

type EmailClient struct {
	sesClient *ses.Client
}

func NewEmailClient() (*EmailClient, error) {
	cfg, err := awsconfig.LoadDefaultConfig(context.TODO(), awsconfig.WithRegion("us-east-1"))
	if err != nil {
		return nil, fmt.Errorf("unable to load AWS config, %v", err)
	}

	svc := ses.NewFromConfig(cfg)

	return &EmailClient{sesClient: svc}, nil
}

func (client *EmailClient) SendEmail(ctx context.Context, recipient, subject, body string) error {
	// logger := utils.GetCtxLogger(ctx)
	from := config.GetString("aws.senderEmail")

	message := &ses.SendEmailInput{
		Destination: &types.Destination{
			ToAddresses: []string{recipient},
		},
		Message: &types.Message{
			Body: &types.Body{
				Text: &types.Content{
					Data: aws.String(body),
				},
			},
			Subject: &types.Content{
				Data: aws.String(subject),
			},
		},
		Source: aws.String(from),
	}

	_, err := client.sesClient.SendEmail(ctx, message)
	if err != nil {
		log.Printf("failed to send email: %v", err)
		return err
	}
	log.Printf("email sent successfully to %s", recipient)
	return nil
}
