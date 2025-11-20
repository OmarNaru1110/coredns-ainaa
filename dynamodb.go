package ainaa

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// DynamoDBRepository implements PersistentRepository using DynamoDB.
type DynamoDBRepository struct {
	client *dynamodb.Client
}

// NewDynamoDBRepository creates a new DynamoDBRepository.
func NewDynamoDBRepository(client *dynamodb.Client) *DynamoDBRepository {
	return &DynamoDBRepository{client: client}
}

func connectDynamoDB(ctx context.Context) (*dynamodb.Client, error) {

	// Load AWS config (reads credentials and region from environment or shared config)
	cfg, err := config.LoadDefaultConfig(ctx)
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

// Get retrieves a domain from DynamoDB.
func (r *DynamoDBRepository) Get(ctx context.Context, domain string) (DomainRecord, error) {
	val, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"domain": &types.AttributeValueMemberS{Value: domain},
		},
	})
	if err != nil {
		return DomainRecord{}, err
	}
	if val.Item == nil {
		return DomainRecord{}, fmt.Errorf("domain not found")
	}

	var domainRecord DomainRecord
	_ = attributevalue.UnmarshalMap(val.Item, &domainRecord)

	return domainRecord, nil
}

// Save stores a domain in DynamoDB.
func (r *DynamoDBRepository) Save(ctx context.Context, record DomainRecord) error {
	item, err := attributevalue.MarshalMap(record)
	if err != nil {
		return err
	}

	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      item,
	})
	return err
}
