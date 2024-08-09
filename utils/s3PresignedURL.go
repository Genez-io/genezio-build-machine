package utils

import (
	"build-machine/internal"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

type reqCreatePresignedURLBody struct {
	ProjectName string `json:"projectName"`
	Region      string `json:"region"`
	Stage       string `json:"stage"`
}
type resCreatePresignedURLBody struct {
	PresignedURL string `json:"presignedURL"`
	Status       string `json:"status"`
	UserId       string `json:"userId"`
	Domain       string `json:"domain"`
}

func UploadContentToS3(archivePath, projectName, region, stage, token string) (string, error) {
	client := &http.Client{}
	if archivePath == "" {
		return "", fmt.Errorf("archivePath is empty")
	}
	// Get the presigned URL from the environment
	presignEndpoint := fmt.Sprintf("%s/core/create-project-code-url", internal.GetConfig().BackendURL)
	body := reqCreatePresignedURLBody{
		ProjectName: projectName,
		Region:      region,
		Stage:       stage,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	reqPresign, err := http.NewRequest("POST", presignEndpoint, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return "", err
	}

	reqPresign.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	reqPresign.Header.Set("Content-Type", "application/json")
	reqPresign.Header.Set("Accept-Version", "genezio-cli/2.0.3")

	res, err := client.Do(reqPresign)
	if err != nil {
		return "", err
	}

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get presigned URL: %s", res.Status)
	}

	var presignedURL string
	resbody := resCreatePresignedURLBody{}
	if err = json.NewDecoder(res.Body).Decode(&resbody); err != nil {
		return "", err
	}

	if resbody.PresignedURL == "" {
		return "", fmt.Errorf("failed to get presigned URL: %s", res.Status)
	}

	presignedURL = resbody.PresignedURL
	// Upload the archive to S3
	f, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}

	fStat, err := f.Stat()
	if err != nil {
		return "", err
	}

	fileBuf := make([]byte, fStat.Size())
	_, err = f.Read(fileBuf)
	if err != nil {
		return "", err
	}

	reqUpload, err := http.NewRequest("PUT", presignedURL, bytes.NewBuffer(fileBuf))
	if err != nil {
		return "", err
	}

	fileSizeStr := fmt.Sprintf("%d", fStat.Size())
	reqUpload.Header.Add("Content-Type", "application/octet-stream")
	reqUpload.Header.Add("Content-Length", fileSizeStr)

	resp, err := client.Do(reqUpload)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to upload archive to S3: %s", resp.Status)
	}

	return presignedURL, nil
}

func DownloadFromS3PresignedURL(region, bucket, key string) (string, error) {
	accessKeyID := internal.GetConfig().AWSAccessKeyID
	secretAccessKey := internal.GetConfig().AWSSecretAccessKey

	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(region),
		Credentials: credentials.NewStaticCredentials(
			accessKeyID,
			secretAccessKey,
			"",
		),
	})

	if err != nil {
		log.Println("Failed to create session", err)
		return "", err
	}
	// Create S3 service client
	svc := s3.New(sess)

	req, _ := svc.GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	urlStr, err := req.Presign(15 * time.Minute)

	if err != nil {
		log.Println("Failed to sign request", err)
		return "", err
	}

	log.Println("The URL is", urlStr)
	return urlStr, nil
}
