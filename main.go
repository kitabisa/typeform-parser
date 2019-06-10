package main

import (
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

	for i, identifier := range hiddenValue {
		// iterate hidden value to determine path-to-save and file name of json file
	}

	data, err := json.Marshal(req.Data)
	if err != nil {
		return "", err
	}

	/*
		Block code for saving data to json file
		still looking for best method to save the response to json file
	*/

	// http://bucket.s3.aws-region.amazonaws.com
	bucket := "http://s3.ap-southeast-1.amazonaws.com/internal-response-staging/"
	filename := "test-aja.json"

	// create a reader from data data in memory
	reader := strings.NewReader(string(data))

	sess, err := session.NewSession(&aws.Config{
		Region: aws.String("ap-southeast-1")},
	)
	uploader := s3manager.NewUploader(sess)

	_, err = uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(filename),
		// here you pass your reader
		// the aws sdk will manage all the memory and file reading for you
		Body: reader,
	})
	if err != nil {
		fmt.Printf("Unable to upload %q to %q, %v", filename, bucket, err)
	}

	fmt.Printf("Successfully uploaded %q to %q\n", filename, bucket)

	return "Test", nil
}

func main() {
	lambda.Start(HandleRequest)
}
