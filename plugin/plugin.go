// Copyright 2020 the Drone Authors. All rights reserved.
// Use of this source code is governed by the Blue Oak Model License
// that can be found in the LICENSE file.

package plugin

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	ststypes "github.com/aws/aws-sdk-go-v2/service/sts/types"
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
	var cfg aws.Config

	if args.Username == "AWS" && args.Password == "" {
		creds, err := getAWSTemporaryCredentials(args.AwsAccessKeyID, args.AwsSecretAcessKey, args.AwsRegion)
		if err != nil {
			return fmt.Errorf("failed to get login to AWS: %w", err)
		}

		cfg, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(args.AwsRegion),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(*creds.AccessKeyId, *creds.SecretAccessKey, *creds.SessionToken)),
		)
		if err != nil {
			return fmt.Errorf("failed to load configuration with temporary credentials: %w", err)
		}
	} else {
		cfg, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(args.AwsRegion),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(args.Username, args.Password, "")),
		)
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}
	}

	smClient := sagemaker.NewFromConfig(cfg)

	if err := createModel(ctx, smClient, args); err != nil {
		return err
	}
	fmt.Println("Model created successfully")

	if err := createEndpointConfig(ctx, smClient, args); err != nil {
		return err
	}
	fmt.Println("Endpoint config created successfully")

	if err := deployEndpoint(ctx, smClient, args); err != nil {
		return err
	}
	fmt.Println("Endpoint deployed successfully")
	fmt.Println("SageMaker deployment completed successfully")

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

func getAWSTemporaryCredentials(accessKeyID, secretAccessKey, region string) (*ststypes.Credentials, error) {
	if accessKeyID == "" || secretAccessKey == "" || region == "" {
		return nil, fmt.Errorf("AWS credentials not provided")
	}

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	stsClient := sts.NewFromConfig(cfg)
	input := &sts.GetSessionTokenInput{}

	result, err := stsClient.GetSessionToken(context.TODO(), input)
	if err != nil {
		return nil, fmt.Errorf("failed to get session token: %w", err)
	}

	logrus.Info("Successfully retrieved AWS temporary credentials\n")

	return result.Credentials, nil
}
