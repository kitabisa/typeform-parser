package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
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

// DocumentStatusResponse for mapping the response
type DocumentStatusResponse struct {
	Data []bool        `json:"data"`
	Meta []interface{} `json:"meta"`
}

// BucketName bucket name on AWS S3
var BucketName = os.Getenv("BUCKETNAME")

// CampaignershipEndpoint endpoint to notify typeform submitted
var CampaignershipEndpoint = os.Getenv("CAMPAIGNERSHIPENDPOINT")

// CampaignershipUsername endpoint to notify typeform submitted
var CampaignershipUsername = os.Getenv("CAMPAIGNERSHIPUSERNAME")

// CampaignershipPassword endpoint to notify typeform submitted
var CampaignershipPassword = os.Getenv("CAMPAIGNERSHIPPASSWORD")

func checkNoPendingSubmission(pathReference string, reference int, statusID int64) (noPendingSubmission bool) {
	endpoint := fmt.Sprintf("%s/check", CampaignershipEndpoint)
	payload := map[string]interface{}{
		"path":        fmt.Sprintf("/%s", pathReference),
		"projects_id": reference,
		"status_id":   0,
	}

	requestByte, err := json.Marshal(payload)
	if err != nil {
		fmt.Println("Failed when marshaling payload (check pending submission on campaignership): ", err)
		return
	}

	client := &http.Client{}
	r, err := http.NewRequest("POST", endpoint, bytes.NewReader(requestByte)) // URL-encoded payload
	if err != nil {
		fmt.Println("Failed when create http request (check pending submission on campaignership): ", err)
		return
	}

	r.SetBasicAuth(CampaignershipUsername, CampaignershipPassword)
	resp, err := client.Do(r)
	if err != nil {
		fmt.Println("failed when send notification to campaign service (check pending submission on campaignership): ", err)
		return
	}

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	var result DocumentStatusResponse
	json.Unmarshal(bodyBytes, &result)

	if result.Data[0] {
		return true
	}

	return
}

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
				referenceValue, _ = strconv.Atoi(identifier.(string))
				fileName = fmt.Sprintf("%s_%d.json", identifier, timestamp)
			}

			if i == "pathreference" {
				pathReference = i
				pathReferenceValue = fmt.Sprintf("%v", identifier)
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

	// check if there is pending submission on DB
	if !checkNoPendingSubmission(pathReferenceValue, referenceValue, 0) {
		fmt.Println("sudah ada submit bos")
		return events.APIGatewayProxyResponse{Body: "There is pending submission. not saving data", StatusCode: 400}, nil
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
	var endpoint string
	var payload map[string]interface{}
	switch pathReferenceValue {
	case "campaign/medical-verification":
		endpoint = CampaignershipEndpoint
		payload = map[string]interface{}{
			"path":        fmt.Sprintf("/%s", pathReferenceValue),
			"projects_id": referenceValue,
			"status_id":   0,
		}

		requestByte, err := json.Marshal(payload)
		if err != nil {
			fmt.Println("error when marshaling struct: ", err)
			return events.APIGatewayProxyResponse{Body: fmt.Sprintf("Successfully uploaded %s to %s, but failed when marshaling paylod when trying to send event typeform submitted (%s:%d)", fileName, bucket, pathReferenceValue, referenceValue), StatusCode: 200}, nil
		}

		client := &http.Client{}
		r, err := http.NewRequest("POST", endpoint, bytes.NewReader(requestByte)) // URL-encoded payload
		if err != nil {
			fmt.Println("failed when create http request: ", err)
			return events.APIGatewayProxyResponse{Body: fmt.Sprintf("Successfully uploaded %s to %s, but failed when create http request (%s:%d)", fileName, bucket, pathReferenceValue, referenceValue), StatusCode: 200}, nil
		}

		r.SetBasicAuth(CampaignershipUsername, CampaignershipPassword)
		resp, err := client.Do(r)
		if err != nil {
			fmt.Println("failed when send notification to campaign service: ", err)
			return events.APIGatewayProxyResponse{Body: fmt.Sprintf("Successfully uploaded %s to %s, but failed when send event typeform submitted (%s:%d)", fileName, bucket, pathReferenceValue, referenceValue), StatusCode: 200}, nil
		}

		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatal(err)
		}
		bodyString := string(bodyBytes)

		return events.APIGatewayProxyResponse{Body: fmt.Sprintf("Successfully uploaded %s to %s, with status code: %d and message: %s", fileName, bucket, resp.StatusCode, bodyString), StatusCode: 200}, nil
		break
	default:
	}

	return events.APIGatewayProxyResponse{Body: fmt.Sprintf("Successfully uploaded %s to %s", fileName, bucket), StatusCode: 200}, nil
}

func main() {
	lambda.Start(HandleRequest)
}
