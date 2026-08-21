package awscfg

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

// Load resolves AWS configuration using the default credential chain
// (environment, shared credentials file, SSO, credential_process, etc.),
// honoring an optional named profile. When region is empty, the region is
// resolved from the environment/profile chain (AWS_REGION, AWS_DEFAULT_REGION,
// or the profile's configured region) rather than being pinned.
func Load(ctx context.Context, region, profile string) (aws.Config, error) {
	var opts []func(*config.LoadOptions) error
	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}
	if profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}
	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return aws.Config{}, fmt.Errorf("load aws config: %w", err)
	}
	return cfg, nil
}
