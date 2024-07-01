package service

import (
	"build-machine/internal"
	"encoding/base64"
	"log"

	"github.com/aws/aws-sdk-go/aws/credentials"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/eks"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/aws-iam-authenticator/pkg/token"
)

type KubernetesClient struct {
	Config *rest.Config
}

func newClientset(cluster *eks.Cluster, sess *session.Session) (*rest.Config, error) {
	gen, err := token.NewGenerator(true, false)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	opts := &token.GetTokenOptions{
		ClusterID: aws.StringValue(cluster.Name),
		Session:   sess,
	}
	tok, err := gen.GetWithOptions(opts)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	ca, err := base64.StdEncoding.DecodeString(aws.StringValue(cluster.CertificateAuthority.Data))
	if err != nil {
		log.Println(err)
		return nil, err
	}

	restConfig := rest.Config{
		Host:        aws.StringValue(cluster.Endpoint),
		BearerToken: tok.Token,
		TLSClientConfig: rest.TLSClientConfig{
			CAData: ca,
		},
	}

	return &restConfig, nil

}

func NewKubernetesConfig() *KubernetesClient {
	accessKeyId := internal.GetConfig().AccessKeyCluster
	accessKeySecret := internal.GetConfig().AccessKeySecretCluster

	clusterName := internal.GetConfig().BuildClusterName
	sess, err := session.NewSessionWithOptions(session.Options{
		Config: aws.Config{
			Region:                        aws.String("us-east-1"),
			CredentialsChainVerboseErrors: aws.Bool(true),
			Credentials:                   credentials.NewStaticCredentials(accessKeyId, accessKeySecret, ""),
		},
	})
	if err != nil {
		log.Printf("Error creating session: %v\n", err)
	}

	eksSvc := eks.New(sess, &aws.Config{})

	input := &eks.DescribeClusterInput{
		Name: aws.String(clusterName),
	}
	result, err := eksSvc.DescribeCluster(input)
	if err != nil {
		log.Printf("Error calling DescribeCluster: %v\n", err)
	}

	restConfig, err := newClientset(result.Cluster, sess)
	if err != nil {
		log.Printf("Error creating clientset: %v\n", err)
	}

	return &KubernetesClient{
		Config: restConfig,
	}
}
