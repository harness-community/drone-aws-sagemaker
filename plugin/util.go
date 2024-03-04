// Copyright 2020 the Drone Authors. All rights reserved.
// Use of this source code is governed by the Blue Oak Model License
// that can be found in the LICENSE file.

package plugin

import (
	"fmt"
)

func verifyArgs(args Args) error {
	if args.ModelName == "" {
		return fmt.Errorf("missing model name")
	}
	if args.ExecutionRoleArn == "" {
		return fmt.Errorf("missing execution role arn")
	}
	if args.ImageURL == "" {
		return fmt.Errorf("missing image url")
	}
	if args.ModelDataUrl == "" {
		return fmt.Errorf("missing model data url")
	}
	if args.EndpointConfigName == "" {
		return fmt.Errorf("missing endpoint config name")
	}
	if args.EndpointName == "" {
		return fmt.Errorf("missing endpoint name")
	}
	if args.InstanceType == "" {
		return fmt.Errorf("missing instance type")
	}
	if args.InitialInstanceCount == 0 {
		return fmt.Errorf("missing initial instance count")
	}
	if args.VariantName == "" {
		return fmt.Errorf("missing variant name")
	}

	return nil
}
