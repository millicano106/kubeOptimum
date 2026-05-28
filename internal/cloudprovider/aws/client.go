package aws

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// NewConfig builds an AWS config from region, named profile, and optional role ARN.
func NewConfig(ctx context.Context, region, profile, roleARN string) (aws.Config, error) {
	slog.Debug("loading AWS config", "region", region, "profile", profile, "role_arn", roleARN)

	opts := []func(*config.LoadOptions) error{
		config.WithRegion(region),
	}
	if profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}

	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return aws.Config{}, fmt.Errorf("load AWS config: %w", err)
	}

	if roleARN != "" {
		slog.Debug("assuming IAM role", "role_arn", roleARN)
		stsClient := sts.NewFromConfig(cfg)
		cfg.Credentials = aws.NewCredentialsCache(
			stscreds.NewAssumeRoleProvider(stsClient, roleARN),
		)
	}

	slog.Debug("AWS config loaded")
	return cfg, nil
}
