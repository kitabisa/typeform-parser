package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

// NotificationPayload for POST request
type NotificationPayload struct {
	PathReference string `json:"path_reference"`
	Reference     int    `json:"reference"`
}

// BucketName bucket name on AWS S3
var BucketName = os.Getenv("BUCKETNAME")

// HandleRequest handle post request from typeform
func HandleRequest(req map[string]interface{}) (events.APIGatewayProxyResponse, error) {
	hiddenValue := req["form_response"].(map[string]interface{})["hidden"].(map[string]interface{})

	var fileName string
	var path string
	var pathReference string
	var reference string
	var pathReferenceValue string
	var referenceValue int
	if hiddenValue != nil {
		timestamp := time.Now().Unix()
		for i, identifier := range hiddenValue {
			// iterate hidden value to determine path-to-save and file name of json file
			if i == "reference" {
				reference = i
				referenceValue = identifier.(int)
				fileName = fmt.Sprintf("%s_%d.json", identifier, timestamp)
			}

			if i == "pathreference" {
				pathReference = i
				pathReference = fmt.Sprintf("%v", identifier)
				path = fmt.Sprintf("/%s", identifier)
			}
		}

		toBeHash := fmt.Sprintf("%s:%s.%s:%s", pathReference, hiddenValue["pathreference"], reference, hiddenValue["reference"])
		hash := sha256.Sum256([]byte(toBeHash))

		if hiddenValue["token"] != hex.EncodeToString(hash[:]) {
			return events.APIGatewayProxyResponse{Body: "Invalid token.", StatusCode: 400}, nil
		}
	} else {
		return events.APIGatewayProxyResponse{Body: "No hidden values specified.", StatusCode: 500}, nil
	}

	bucket := fmt.Sprintf("%s/%s", BucketName, path)

	data, err := json.Marshal(req)
	if err != nil {
		return events.APIGatewayProxyResponse{Body: "Failed when marshal data.", StatusCode: 500}, err
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
		return events.APIGatewayProxyResponse{Body: fmt.Sprintf("Unable to upload %s to %s, %v", fileName, bucket, err), StatusCode: 500}, err
	}

	// send http request to notify services
	var campaignEndpoint string
	switch pathReferenceValue {
	case "campaign/medical-verification":
		campaignEndpoint = "https://campaign.ktbs.io/typeform-submit-notification/"
		break
	default:
	}

	payload := NotificationPayload{
		PathReference: pathReferenceValue,
		Reference:     referenceValue,
	}

	requestByte, err := json.Marshal(payload)
	if err != nil {
		fmt.Println("error when marshaling struct: ", err)
	}

	client := &http.Client{}
	r, err := http.NewRequest("POST", campaignEndpoint, bytes.NewReader(requestByte)) // URL-encoded payload

	resp, err := client.Do(r)
	if err != nil {
		fmt.Println("failed when send notification to campaign service: ", err)
	}

	fmt.Println((resp))

	return events.APIGatewayProxyResponse{Body: fmt.Sprintf("Successfully uploaded %s to %s", fileName, bucket), StatusCode: 200}, nil
}

func main() {
	lambda.Start(HandleRequest)
}
