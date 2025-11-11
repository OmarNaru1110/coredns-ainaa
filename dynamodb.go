package ainaa

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

func connectDynamoDB(ctx context.Context) (*dynamodb.Client, error) {

	// Load AWS config (reads credentials and region from environment or shared config)
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("us-west-2"), // any region name, DynamoDB Local doesn’t care
		config.WithEndpointResolverWithOptions(
			aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				if service == dynamodb.ServiceID {
					return aws.Endpoint{
						URL:               "http://localhost:8000", // DynamoDB Local default
						HostnameImmutable: true,
					}, nil
				}
				return aws.Endpoint{}, fmt.Errorf("unknown endpoint requested")
			}),
		),
	)
	if err != nil {
		log.Fatalf("Failed to load AWS config: %v", err)
	}

	client := dynamodb.NewFromConfig(cfg)

	// check the connection with a light call (for readiness)
	_, err = client.ListTables(ctx, &dynamodb.ListTablesInput{Limit: aws.Int32(1)})
	if err != nil {
		return nil, err
	}

	return client, nil
}
