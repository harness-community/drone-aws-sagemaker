// Copyright 2020 the Drone Authors. All rights reserved.
// Use of this source code is governed by the Blue Oak Model License
// that can be found in the LICENSE file.

package plugin

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker/types"
	"github.com/sirupsen/logrus"
)

// Args provides plugin execution arguments.
type Args struct {
	Pipeline

	// Level defines the plugin log level.
	Level string `envconfig:"PLUGIN_LOG_LEVEL"`

	// TODO replace or remove
	ModelName            string `envconfig:"PLUGIN_MODEL_NAME"`
	ExecutionRoleArn     string `envconfig:"PLUGIN_EXECUTION_ROLE_ARN"`
	ImageURL             string `envconfig:"PLUGIN_IMAGE_URL"`
	ModelDataUrl         string `envconfig:"PLUGIN_MODEL_DATA_URL"`
	EndpointConfigName   string `envconfig:"PLUGIN_ENDPOINT_CONFIG_NAME"`
	EndpointName         string `envconfig:"PLUGIN_ENDPOINT_NAME"`
	InstanceType         string `envconfig:"PLUGIN_INSTANCE_TYPE"`
	InitialInstanceCount int64  `envconfig:"PLUGIN_INITIAL_INSTANCE_COUNT"`
	VariantName          string `envconfig:"PLUGIN_VARIANT_NAME"`

	AwsAccessKeyID    string `envconfig:"PLUGIN_AWS_ACCESS_KEY_ID"`
	AwsSecretAcessKey string `envconfig:"PLUGIN_AWS_SECRET_ACCESS_KEY"`
	AwsRegion         string `envconfig:"PLUGIN_AWS_REGION"`
	Username          string `envconfig:"PLUGIN_USERNAME"`
	Password          string `envconfig:"PLUGIN_PASSWORD"`
}

func Exec(ctx context.Context, args Args) error {
	if err := verifyArgs(args); err != nil {
		return err
	}

	var err error

	if args.Username == "AWS" && args.Password == "" {
		args.Password, err = getAWSPassword(args.AwsAccessKeyID, args.AwsSecretAcessKey, args.AwsRegion)
		if err != nil {
			return fmt.Errorf("failed to get login to AWS")
		}
	}

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(args.AwsRegion),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(args.Username, args.Password, "")),
	)

	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	smClient := sagemaker.NewFromConfig(cfg)

	if err := createModel(ctx, smClient, args); err != nil {
		return err
	}
	logrus.Info("Model created successfully")

	if err := createEndpointConfig(ctx, smClient, args); err != nil {
		return err
	}
	logrus.Info("Endpoint config created successfully")

	if err := deployEndpoint(ctx, smClient, args); err != nil {
		return err
	}
	logrus.Info("Endpoint deployed successfully")

	return nil
}

func createModel(ctx context.Context, smClient *sagemaker.Client, args Args) error {
	modelInput := &sagemaker.CreateModelInput{
		ExecutionRoleArn: aws.String(args.ExecutionRoleArn),
		ModelName:        aws.String(args.ModelName),
		PrimaryContainer: &types.ContainerDefinition{
			Image:        aws.String(args.ImageURL),
			ModelDataUrl: aws.String(args.ModelDataUrl),
		},
	}

	_, err := smClient.CreateModel(ctx, modelInput)
	if err != nil {
		return fmt.Errorf("failed to create model: %w", err)
	}
	return nil
}

func createEndpointConfig(ctx context.Context, smClient *sagemaker.Client, args Args) error {
	endpointConfigInput := &sagemaker.CreateEndpointConfigInput{
		EndpointConfigName: aws.String(args.EndpointConfigName),
		ProductionVariants: []types.ProductionVariant{
			{
				InstanceType:         types.ProductionVariantInstanceType(args.InstanceType),
				InitialInstanceCount: aws.Int32(int32(args.InitialInstanceCount)),
				ModelName:            aws.String(args.ModelName),
				VariantName:          aws.String(args.VariantName),
			},
		},
	}

	_, err := smClient.CreateEndpointConfig(ctx, endpointConfigInput)
	if err != nil {
		return fmt.Errorf("failed to create endpoint config: %w", err)
	}
	return nil
}

func deployEndpoint(ctx context.Context, smClient *sagemaker.Client, args Args) error {
	endpointInput := &sagemaker.CreateEndpointInput{
		EndpointConfigName: aws.String(args.EndpointConfigName),
		EndpointName:       aws.String(args.EndpointName),
	}

	_, err := smClient.CreateEndpoint(ctx, endpointInput)
	if err != nil {
		return fmt.Errorf("failed to create endpoint: %w", err)
	}
	return nil
}

func getAWSPassword(accessKeyID, secretAccessKey, region string) (string, error) {
	if accessKeyID == "" || secretAccessKey == "" || region == "" {
		return "", fmt.Errorf("aws credentials not provided")
	}

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, "")),
	)
	if err != nil {
		return "", fmt.Errorf("failed to load aws config: %w", err)
	}

	svc := ecr.NewFromConfig(cfg)

	input := &ecr.GetAuthorizationTokenInput{}
	result, err := svc.GetAuthorizationToken(context.TODO(), input)
	if err != nil {
		fmt.Println("Error getting authorization token:", err)
		return "", err
	}

	var awsToken string

	for _, data := range result.AuthorizationData {
		token, err := base64.StdEncoding.DecodeString(*data.AuthorizationToken)
		if err != nil {
			fmt.Println("Error decoding token:", err)
			return "", err
		}

		awsToken = string(token)
	}

	logrus.Info("successfully retrieved aws token\n")

	return strings.Split(awsToken, ":")[1], nil
}
