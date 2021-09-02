/*
Copyright 2021 k0s authors

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
package config

import (
	"context"
	"fmt"
	"strings"
	"time"

	cfgClient "github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/clientset"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0s/pkg/constant"

	"github.com/sirupsen/logrus"
)

var (
	resourceType = v1.TypeMeta{APIVersion: "k0s.k0sproject.io/v1beta1", Kind: "clusterconfigs"}
	getOpts      = v1.GetOptions{TypeMeta: resourceType}
)

func GetConfigFromAPI(kubeConfig string) (*v1beta1.ClusterConfig, error) {
	timeout := time.After(20 * time.Second)
	tick := time.Tick(3 * time.Second)
	// Keep trying until we're timed out or got a result or got an error
	for {
		select {
		// Got a timeout! fail with a timeout error
		case <-timeout:
			return nil, fmt.Errorf("timed out waiting for API to return cluster-config")
		// Got a tick, we should check on doSomething()
		case <-tick:
			logrus.Debug("fetching cluster-config from API...")
			cfg, err := configRequest(kubeConfig)
			if err != nil {
				continue
			}
			return cfg, nil
		}
	}
}

// fetch cluster-config from API
func configRequest(kubeConfig string) (clusterConfig *v1beta1.ClusterConfig, err error) {
	c, err := cfgClient.NewForConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("can't create kubernetes typed client for cluster config: %v", err)
	}
	configClient := c.ClusterConfigs(constant.ClusterConfigNamespace)
	ctxWithTimeout, cancelFunction := context.WithTimeout(context.Background(), time.Duration(10)*time.Second)
	defer cancelFunction()

	cfg, err := configClient.Get(ctxWithTimeout, "k0s", getOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch cluster-config from API: %v", err)
	}
	return cfg, nil
}

func GetYamlFromFile(cfgPath string, k0sVars constant.CfgVars) (clusterConfig *v1beta1.ClusterConfig, err error) {
	if cfgPath == "" {
		// no config file exists, using defaults
		logrus.Info("no config file given, using defaults")
	}
	cfg, err := ValidateYaml(cfgPath, k0sVars)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func ValidateYaml(cfgPath string, k0sVars constant.CfgVars) (clusterConfig *v1beta1.ClusterConfig, err error) {
	switch cfgPath {
	case "-":
		clusterConfig, err = v1beta1.ConfigFromStdin(k0sVars.DataDir)
	case "":
		clusterConfig = v1beta1.DefaultClusterConfig(k0sVars.DataDir)
	default:
		clusterConfig, err = v1beta1.ConfigFromFile(cfgPath, k0sVars.DataDir)
	}
	if err != nil {
		return nil, err
	}

	if clusterConfig.Spec.Storage.Type == v1beta1.KineStorageType && clusterConfig.Spec.Storage.Kine == nil {
		clusterConfig.Spec.Storage.Kine = v1beta1.DefaultKineConfig(k0sVars.DataDir)
	}
	if clusterConfig.Spec.Install == nil {
		clusterConfig.Spec.Install = v1beta1.DefaultInstallSpec()
	}

	errors := clusterConfig.Validate()
	if len(errors) > 0 {
		messages := make([]string, len(errors))
		for _, e := range errors {
			messages = append(messages, e.Error())
		}
		return nil, fmt.Errorf(strings.Join(messages, "\n"))
	}
	return clusterConfig, nil
}
