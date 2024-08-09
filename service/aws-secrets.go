package service

import (
	"build-machine/internal"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
)

func Encrypt(key []byte, value []byte) ([]byte, error) {
	// check if value is empty
	if len(value) == 0 {
		return []byte{}, errors.New("value to be encrypted is empty")
	}

	aesBlock, err := aes.NewCipher(key)
	if err != nil {
		return []byte{}, err
	}

	gcmInstance, err := cipher.NewGCM(aesBlock)
	if err != nil {
		return []byte{}, err
	}

	nonce := make([]byte, gcmInstance.NonceSize())
	_, err = io.ReadFull(rand.Reader, nonce)
	if err != nil {
		return []byte{}, err
	}

	cipheredText := gcmInstance.Seal(nonce, nonce, value, nil)

	return cipheredText, nil
}

func Decrypt(key []byte, ciphered []byte) ([]byte, error) {
	// check if value is empty
	if len(ciphered) == 0 {
		return []byte{}, errors.New("value to be decrypted is empty")
	}

	aesBlock, err := aes.NewCipher(key)
	if err != nil {
		return []byte{}, err
	}

	gcmInstance, err := cipher.NewGCM(aesBlock)
	if err != nil {
		return []byte{}, err
	}

	nonceSize := gcmInstance.NonceSize()
	nonce, cipheredText := ciphered[:nonceSize], ciphered[nonceSize:]

	plainText, err := gcmInstance.Open(nil, nonce, cipheredText, nil)
	if err != nil {
		return []byte{}, err
	}

	return plainText, nil
}

func getSecretFromAWSSecretManager() (map[string]string, error) {
	accessKeyId := internal.GetConfig().AWSAccessKeyID
	accessKeySecret := internal.GetConfig().AWSSecretAccessKey

	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String("us-east-1"),
		Credentials: credentials.NewStaticCredentials(accessKeyId, accessKeySecret, ""),
	})
	if err != nil {
		log.Println("New session failed! with error", err)
		return nil, err
	}

	svc := secretsmanager.New(sess)

	secretPath := "genezio-dev-environment-variables"
	input := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretPath),
	}

	result, err := svc.GetSecretValue(input)
	if err != nil {
		log.Println("Get secret value failed! with error", err, secretPath)
		return nil, err
	}
	secretString := *result.SecretString

	var secretsMap map[string]string
	err = json.Unmarshal([]byte(secretString), &secretsMap)
	if err != nil {
		return nil, err
	}

	return secretsMap, nil
}

type GetEnvVarsCoreResponse struct {
	Status               string `json:"status"`
	EnvironmentVariables []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"environmentVariables"`
}

func GetEnvVarsByStageID(stageId string) (envVars map[string]string, err error) {
	secretsMap, err := getSecretFromAWSSecretManager()
	if err != nil {
		log.Println("Failed to get secrets from AWS Secret Manager", err)
		return nil, err
	}

	decodeKey := []byte(secretsMap[internal.GetConfig().EnvVarsSecretName])

	// Send GET http request to backend api
	route := fmt.Sprintf("%s/projects/%s/environment-variables/build-machine", internal.GetConfig().BackendURL, stageId)
	req, err := http.NewRequest("GET", route, nil)
	if err != nil {
		log.Println("Failed to create request", err)
		return nil, err
	}

	// Add basic auth
	req.SetBasicAuth(internal.GetConfig().BasicCoreUsername, internal.GetConfig().BasicCorePassword)
	req.Header.Set("Accept-Version", "genezio-cli/2.0.3")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Failed to send request", err)
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get env vars: %s", resp.Status)
	}

	var envVarsCore GetEnvVarsCoreResponse
	if err = json.NewDecoder(resp.Body).Decode(&envVarsCore); err != nil {
		return nil, err
	}

	envVars = make(map[string]string)
	for _, envVar := range envVarsCore.EnvironmentVariables {
		// Decode value from base64
		valB64Decoded, err := base64.StdEncoding.DecodeString(envVar.Value)
		if err != nil {
			log.Println("Failed to decode base64", err)
			return nil, err
		}

		// Decrypt value
		valDecrypted, err := Decrypt(decodeKey, valB64Decoded)
		if err != nil {
			log.Println("Failed to decrypt", err)
			return nil, err
		}

		envVars[envVar.Name] = string(valDecrypted)
	}
	return envVars, nil
}
