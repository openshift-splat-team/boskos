/*
Copyright 2024 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	configv2 "github.com/aws/aws-sdk-go-v2/config"
	"github.com/sirupsen/logrus"

	"sigs.k8s.io/boskos/aws-janitor/account"
	"sigs.k8s.io/boskos/aws-janitor/regions"
	"sigs.k8s.io/boskos/aws-janitor/resources"
	s3path "sigs.k8s.io/boskos/aws-janitor/s3"
)

// Event represents the Lambda event input
type Event struct {
	TTL                        string   `json:"ttl"`
	Path                       string   `json:"path"`
	Region                     string   `json:"region"`
	CleanAll                   bool     `json:"cleanAll"`
	DryRun                     bool     `json:"dryRun"`
	IncludeTags                []string `json:"includeTags"`
	ExcludeTags                []string `json:"excludeTags"`
	TTLTagKey                  string   `json:"ttlTagKey"`
	EnableTargetGroupClean     bool     `json:"enableTargetGroupClean"`
	EnableKeyPairsClean        bool     `json:"enableKeyPairsClean"`
	EnableVPCEndpointsClean    bool     `json:"enableVPCEndpointsClean"`
	SkipRoute53ManagementCheck bool     `json:"skipRoute53ManagementCheck"`
	EnableDNSZoneClean         bool     `json:"enableDNSZoneClean"`
	EnableS3BucketsClean       bool     `json:"enableS3BucketsClean"`
	SkipIAMClean               bool     `json:"skipIAMClean"`
	CleanEcrRepositories       []string `json:"cleanEcrRepositories"`
	SkipResourceRecordSetTypes []string `json:"skipResourceRecordSetTypes"`
}

// Response represents the Lambda response
type Response struct {
	Message       string `json:"message"`
	SweptCount    int    `json:"sweptCount"`
	DryRun        bool   `json:"dryRun"`
	ExecutionTime string `json:"executionTime"`
}

func handleRequest(ctx context.Context, event Event) (Response, error) {
	startTime := time.Now()

	// Set log level from environment
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		return Response{}, fmt.Errorf("invalid log level: %w", err)
	}
	logrus.SetLevel(level)

	logrus.Info("AWS Janitor Lambda starting...")

	// Parse TTL
	var maxTTL time.Duration
	if event.TTL != "" {
		maxTTL, err = time.ParseDuration(event.TTL)
		if err != nil {
			return Response{}, fmt.Errorf("invalid TTL: %w", err)
		}
	} else {
		maxTTL = 24 * time.Hour
	}

	// Load AWS config
	config, err := configv2.LoadDefaultConfig(ctx,
		configv2.WithRegion(regions.Default),
		configv2.WithRetryMaxAttempts(100),
	)
	if err != nil {
		return Response{}, fmt.Errorf("failed loading AWS config: %w", err)
	}

	// Get account
	acct, err := account.GetAccount(config, regions.Default)
	if err != nil {
		return Response{}, fmt.Errorf("failed retrieving account: %w", err)
	}
	logrus.Debugf("account: %s", acct)

	// Process exclude tags (always add preserve)
	excludeTags := event.ExcludeTags
	preserveTagFound := false
	for _, tag := range excludeTags {
		if tag == "preserve" || strings.HasPrefix(tag, "preserve=") {
			preserveTagFound = true
			break
		}
	}
	if !preserveTagFound {
		excludeTags = append(excludeTags, "preserve")
		logrus.Info("Automatically excluding resources with 'preserve' tag")
	}

	// Parse tag matchers
	excludeTM, err := resources.TagMatcherForTags(excludeTags)
	if err != nil {
		return Response{}, fmt.Errorf("error parsing exclude tags: %w", err)
	}
	includeTM, err := resources.TagMatcherForTags(event.IncludeTags)
	if err != nil {
		return Response{}, fmt.Errorf("error parsing include tags: %w", err)
	}

	// Process resource record set types
	skipResourceRecordSetTypesSet := map[string]bool{}
	if len(event.SkipResourceRecordSetTypes) == 0 {
		event.SkipResourceRecordSetTypes = []string{"SOA", "NS"}
	}
	for _, resourceRecordType := range event.SkipResourceRecordSetTypes {
		skipResourceRecordSetTypesSet[resourceRecordType] = true
	}

	// Build options
	opts := resources.Options{
		Config:                     &config,
		Account:                    acct,
		DryRun:                     event.DryRun,
		ExcludeTags:                excludeTM,
		IncludeTags:                includeTM,
		TTLTagKey:                  event.TTLTagKey,
		EnableTargetGroupClean:     event.EnableTargetGroupClean,
		EnableKeyPairsClean:        event.EnableKeyPairsClean,
		EnableVPCEndpointsClean:    event.EnableVPCEndpointsClean,
		SkipRoute53ManagementCheck: event.SkipRoute53ManagementCheck,
		EnableDNSZoneClean:         event.EnableDNSZoneClean,
		EnableS3BucketsClean:       event.EnableS3BucketsClean,
		SkipIAMClean:               event.SkipIAMClean,
		CleanEcrRepositories:       event.CleanEcrRepositories,
		SkipResourceRecordSetTypes: skipResourceRecordSetTypesSet,
	}

	var sweepCount int

	// Execute cleanup
	if event.CleanAll {
		logrus.Info("Running in CleanAll mode")
		if err := resources.CleanAll(opts, event.Region); err != nil {
			return Response{}, fmt.Errorf("error cleaning all resources: %w", err)
		}
	} else {
		logrus.Info("Running in MarkAndSweep mode")
		if event.Path == "" {
			return Response{}, fmt.Errorf("--path is required for mark-and-sweep mode")
		}

		s3p, err := s3path.GetPath(opts.Config, event.Path)
		if err != nil {
			return Response{}, fmt.Errorf("invalid S3 path %q: %w", event.Path, err)
		}

		// Check state bucket isn't managed
		if event.EnableS3BucketsClean {
			isManagedBucket, err := resources.IsManagedS3Bucket(opts, s3p.Region, s3p.Bucket)
			if err != nil {
				return Response{}, fmt.Errorf("error checking bucket %s: %w", s3p.Bucket, err)
			}
			if isManagedBucket {
				return Response{}, fmt.Errorf("state bucket %s must be tagged with exclude-tags", s3p.Bucket)
			}
		}

		// Parse regions
		regionList, err := regions.ParseRegion(opts.Config, event.Region)
		if err != nil {
			return Response{}, err
		}
		logrus.Infof("Regions: %+v", regionList)

		// Load state
		res, err := resources.LoadSet(opts.Config, s3p, maxTTL)
		if err != nil {
			return Response{}, fmt.Errorf("error loading state from %q: %w", event.Path, err)
		}

		// Clean regional resources
		for _, region := range regionList {
			opts.Region = region
			for _, typ := range resources.RegionalTypeList {
				if err := typ.MarkAndSweep(opts, res); err != nil {
					return Response{}, fmt.Errorf("error sweeping %T: %w", typ, err)
				}
			}
		}

		// Clean global resources
		opts.Region = regions.Default
		for _, typ := range resources.GlobalTypeList {
			if opts.SkipIAMClean && resources.IsIAMType(typ) {
				logrus.Debugf("Skipping IAM resource type %T", typ)
				continue
			}
			if err := typ.MarkAndSweep(opts, res); err != nil {
				return Response{}, fmt.Errorf("error sweeping %T: %w", typ, err)
			}
		}

		// Mark complete and save
		sweepCount = res.MarkComplete()
		if err := res.Save(opts.Config, s3p); err != nil {
			return Response{}, fmt.Errorf("error saving state to %q: %w", event.Path, err)
		}

		logrus.Infof("swept %d resources", sweepCount)
	}

	executionTime := time.Since(startTime)
	return Response{
		Message:       "AWS Janitor completed successfully",
		SweptCount:    sweepCount,
		DryRun:        event.DryRun,
		ExecutionTime: executionTime.String(),
	}, nil
}

func main() {
	lambda.Start(handleRequest)
}
