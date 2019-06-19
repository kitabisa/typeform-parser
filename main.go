package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

type Request struct {
	Data map[string]interface{} `json:"data"`
}

func HandleRequest(req Request) (string, error) {
	hiddenValue, _ := req.Data["form_response"].(map[string]interface{})["hidden"].(map[string]interface{})

	var fileName string
	var path string
	for i, identifier := range hiddenValue {
		// iterate hidden value to determine path-to-save and file name of json file
		if i == "id" {
			fileName = fmt.Sprintf("%s.json", identifier)
		}

		if i == "model" {
			path = fmt.Sprintf("%s/%s", path, identifier)
		}
	}

	toBeHash := fmt.Sprintf("model:%s.id:%s", hiddenValue["model"], hiddenValue["id"])

	fmt.Println(toBeHash)

	hash := sha256.Sum256([]byte(toBeHash))

	fmt.Println(hex.EncodeToString(hash[:]))

	if hiddenValue["token"] != hex.EncodeToString(hash[:]) {
		return "token is not valid", nil
	}

	bucket := fmt.Sprintf("internal-response-staging/%s", path)

	data, err := json.Marshal(req.Data)
	if err != nil {
		return "", err
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
		fmt.Printf("Unable to upload %q to %q, %v", fileName, bucket, err)
	}

	fmt.Printf("Successfully uploaded %q to %q\n", fileName, bucket)

	return "Test", nil
}

func main() {
	lambda.Start(HandleRequest)
}
