package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

const (
	// BucketName for save response typeform
	BucketName = "internal-response-staging"
)

func HandleRequest(req map[string]interface{}) (string, error) {
	hiddenValue, _ := req["form_response"].(map[string]interface{})["hidden"].(map[string]interface{})

	var fileName string
	var path string
	var pathReference string
	var reference string
	timestamp := time.Now().Unix()
	for i, identifier := range hiddenValue {
		// iterate hidden value to determine path-to-save and file name of json file
		if i == "reference" {
			reference = i
			fileName = fmt.Sprintf("%s_%d.json", identifier, timestamp)
		}

		if i == "pathreference" {
			pathReference = i
			path = fmt.Sprintf("/%s", identifier)
		}
	}

	toBeHash := fmt.Sprintf("%s:%s.%s:%s", pathReference, hiddenValue["pathreference"], reference, hiddenValue["reference"])

	hash := sha256.Sum256([]byte(toBeHash))

	if hiddenValue["token"] != hex.EncodeToString(hash[:]) {
		return "invalid token", nil
	}

	bucket := fmt.Sprintf("%s/%s", BucketName, path)

	data, err := json.Marshal(req)
	if err != nil {
		return "invalid to marshal data", err
	}

	// create a reader from data data in memory
	reader := strings.NewReader(string(data))

	sess, err := session.NewSession(&aws.Config{
		Region: aws.String("ap-southeast-1")},
	)
	uploader := s3manager.NewUploader(sess)

	_, err = uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(fileName),
		Body:   reader,
	})
	if err != nil {
		return fmt.Sprintf("Unable to upload %s to %s, %v", fileName, bucket, err), nil
	}

	return fmt.Sprintf("Successfully uploaded %s to %s", fileName, bucket), nil
}

func main() {
	lambda.Start(HandleRequest)
}
