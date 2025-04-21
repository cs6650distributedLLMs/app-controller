package clients

import (
	"context"
	"encoding/base64"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"tldr/internal/core"
	"tldr/internal/models"
)

var (
	DynamoDBClient *dynamodb.Client
	TableName      string
)

// Initialize DynamoDB client with default settings
func InitDyanmoDb() {
	// Try to get table name from environment, or use default
	TableName = os.Getenv("DOCUMENT_DYNAMODB_TABLE_NAME")

	// Configure the DynamoDB client
	var cfg aws.Config
	var err error

	cfg, err = config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(os.Getenv("AWS_REGION")),
	)

	if err != nil {
		panic(err)
	}

	// Create the DynamoDB client
	DynamoDBClient = dynamodb.NewFromConfig(cfg)
}

// CreateDocument stores metadata about a document object in DynamoDB
func CreateDocument(ctx context.Context, document *models.Document) error {
	now := time.Now()

	item := map[string]types.AttributeValue{
		"PK":        &types.AttributeValueMemberS{Value: "DOCUMENT#" + document.DocumentID},
		"SK":        &types.AttributeValueMemberS{Value: "METADATA"},
		"size":      &types.AttributeValueMemberN{Value: strconv.FormatInt(document.Size, 10)},
		"name":      &types.AttributeValueMemberS{Value: document.Name},
		"mimetype":  &types.AttributeValueMemberS{Value: document.MimeType},
		"status":    &types.AttributeValueMemberS{Value: string(document.Status)},
		"createdAt": &types.AttributeValueMemberS{Value: now.Format(time.RFC3339)},
		"updatedAt": &types.AttributeValueMemberS{Value: now.Format(time.RFC3339)},
	}

	// Add content if provided
	if document.Content != nil {
		// Convert to base64 for storage
		contentBase64 := base64.StdEncoding.EncodeToString(document.Content)
		item["content"] = &types.AttributeValueMemberS{Value: contentBase64}
	}

	_, err := DynamoDBClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(TableName),
		Item:      item,
	})
	return err
}

// GetDocument gets metadata about a document object from DynamoDB
func GetDocument(ctx context.Context, documentID string) (*models.Document, error) {
	result, err := DynamoDBClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(TableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: "DOCUMENT#" + documentID},
			"SK": &types.AttributeValueMemberS{Value: "METADATA"},
		},
	})
	if err != nil {
		return nil, err
	}

	if result.Item == nil {
		return nil, nil
	}

	var document models.Document
	document.DocumentID = documentID

	if v, ok := result.Item["size"].(*types.AttributeValueMemberN); ok {
		sizeVal, err := strconv.ParseInt(v.Value, 10, 64)
		if err != nil {
			return nil, err
		}
		document.Size = sizeVal
	}

	if v, ok := result.Item["name"].(*types.AttributeValueMemberS); ok {
		document.Name = v.Value
	}

	if v, ok := result.Item["mimetype"].(*types.AttributeValueMemberS); ok {
		document.MimeType = v.Value
	}

	if v, ok := result.Item["status"].(*types.AttributeValueMemberS); ok {
		document.Status = core.DocumentStatus(v.Value)
	}

	// Retrieve content if available
	if v, ok := result.Item["content"].(*types.AttributeValueMemberS); ok {
		// Decode from base64
		content, err := base64.StdEncoding.DecodeString(v.Value)
		if err == nil {
			document.Content = content
		}
	}

	return &document, nil
}

// SetDocumentStatus updates the status of a document object in DynamoDB
func SetDocumentStatus(ctx context.Context, documentID string, newStatus core.DocumentStatus) error {
	now := time.Now()

	_, err := DynamoDBClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(TableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: "DOCUMENT#" + documentID},
			"SK": &types.AttributeValueMemberS{Value: "METADATA"},
		},
		UpdateExpression: aws.String("SET #status = :newStatus, #updatedAt = :updatedAt"),
		ExpressionAttributeNames: map[string]string{
			"#status":    "status",
			"#updatedAt": "updatedAt",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":newStatus": &types.AttributeValueMemberS{Value: string(newStatus)},
			":updatedAt": &types.AttributeValueMemberS{Value: now.Format(time.RFC3339)},
		},
	})
	return err
}

// SetDocumentStatusConditionally conditionally updates the status of a document object in DynamoDB
func SetDocumentStatusConditionally(ctx context.Context, documentID string, newStatus, expectedCurrentStatus core.DocumentStatus) (*models.Document, error) {
	now := time.Now()

	result, err := DynamoDBClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(TableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: "DOCUMENT#" + documentID},
			"SK": &types.AttributeValueMemberS{Value: "METADATA"},
		},
		UpdateExpression:    aws.String("SET #status = :newStatus, #updatedAt = :updatedAt"),
		ConditionExpression: aws.String("#status = :expectedCurrentStatus"),
		ExpressionAttributeNames: map[string]string{
			"#status":    "status",
			"#updatedAt": "updatedAt",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":newStatus":             &types.AttributeValueMemberS{Value: string(newStatus)},
			":expectedCurrentStatus": &types.AttributeValueMemberS{Value: string(expectedCurrentStatus)},
			":updatedAt":             &types.AttributeValueMemberS{Value: now.Format(time.RFC3339)},
		},
		ReturnValues: types.ReturnValueAllNew,
	})
	if err != nil {
		return nil, err
	}

	if result.Attributes == nil {
		return nil, nil
	}

	var name string
	if v, ok := result.Attributes["name"].(*types.AttributeValueMemberS); ok {
		name = v.Value
	}

	return &models.Document{
		Name: name,
	}, nil
}

// DeleteDocument deletes the document object metadata from DynamoDB
func DeleteDocument(ctx context.Context, documentID string) (*models.Document, error) {
	result, err := DynamoDBClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(TableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: "DOCUMENT#" + documentID},
			"SK": &types.AttributeValueMemberS{Value: "METADATA"},
		},
		ReturnValues: types.ReturnValueAllOld,
	})
	if err != nil {
		return nil, err
	}

	if result.Attributes == nil {
		return nil, nil
	}

	var name string
	var status core.DocumentStatus

	if v, ok := result.Attributes["name"].(*types.AttributeValueMemberS); ok {
		name = v.Value
	}

	if v, ok := result.Attributes["status"].(*types.AttributeValueMemberS); ok {
		status = core.DocumentStatus(v.Value)
	}

	return &models.Document{
		Name:   name,
		Status: status,
	}, nil
}

// GetAllDocument retrieves all document items from DynamoDB
func GetAllDocument(ctx context.Context) ([]models.DocumentListItem, error) {
	result, err := DynamoDBClient.Scan(ctx, &dynamodb.ScanInput{
		TableName:        aws.String(TableName),
		FilterExpression: aws.String("SK = :sk AND attribute_exists(#name)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":sk": &types.AttributeValueMemberS{Value: "METADATA"},
		},
		ExpressionAttributeNames: map[string]string{
			"#name": "name",
		},
	})
	if err != nil {
		return nil, err
	}

	var documentList []models.DocumentListItem
	for _, item := range result.Items {
		var document models.DocumentListItem

		if v, ok := item["PK"].(*types.AttributeValueMemberS); ok {
			// Extract documentId from PK, which is in format "DOCUMENT#{documentId}"
			document.DocumentID = v.Value[9:] // Skip "DOCUMENT#" prefix
		}

		if v, ok := item["size"].(*types.AttributeValueMemberN); ok {
			sizeVal, err := strconv.ParseInt(v.Value, 10, 64)
			if err != nil {
				continue // Skip this item if size is invalid
			}
			document.Size = sizeVal
		}

		if v, ok := item["name"].(*types.AttributeValueMemberS); ok {
			document.Name = v.Value
		}

		if v, ok := item["mimetype"].(*types.AttributeValueMemberS); ok {
			document.MimeType = v.Value
		}

		documentList = append(documentList, document)
	}

	return documentList, nil
}

// GetPDFContent retrieves the PDF content for a document item
func GetPDFContent(ctx context.Context, documentID string) ([]byte, error) {
	// Get the document item
	document, err := GetDocument(ctx, documentID)
	if err != nil {
		return nil, err
	}

	if document == nil {
		return nil, nil
	}

	return document.Content, nil
}
