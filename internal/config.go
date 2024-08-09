package internal

import (
	"log"
	"os"
	"reflect"

	"github.com/joho/godotenv"
)

type configStruct struct {
	ServerPort          string `key:"SERVER_PORT" default:"8080"`
	BackendURL          string `key:"BACKEND_URL" default:"https://dev.api.genez.io"`
	AWSAccessKeyID      string `key:"AWS_ACCESS_KEY_ID"`
	AWSSecretAccessKey  string `key:"AWS_SECRET_ACCESS_KEY"`
	BucketBaseName      string `key:"BUCKET_BASE_NAME"`
	MaxConcurrentBuilds string `key:"MAX_CONCURRENT_BUILDS" default:"3"`
	// Environment
	Env string `key:"ENV" default:"local"`

	// Kubernetes
	AccessKeyCluster       string `key:"ACCESS_KEY_CLUSTER"`
	AccessKeySecretCluster string `key:"ACCESS_KEY_SECRET_CLUSTER"`
	BuildClusterName       string `key:"BUILD_CLUSTER_NAME"`
}

var config *configStruct

func loadConfigFromEnv() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file", err)
	}

	// Get array of fields for the config struct
	config = &configStruct{}
	res := reflect.VisibleFields(reflect.TypeOf(*config))

	for _, field := range res {
		fieldKeyTag := field.Tag.Get("key")
		fieldValue := os.Getenv(fieldKeyTag)
		if fieldValue == "" {
			fieldValue = field.Tag.Get("default")
		}

		reflect.ValueOf(config).Elem().FieldByName(field.Name).SetString(fieldValue)
	}
}

func GetConfig() *configStruct {
	if config == nil {
		loadConfigFromEnv()
	}

	return config
}
