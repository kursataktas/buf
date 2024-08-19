// Copyright 2020-2024 Buf Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bufcheck

import (
	"github.com/bufbuild/bufplugin-go/check"
)

// checkClientSpec contains a check.Client and details on what to do about
// options it should pass when calling check.
//
// This allows us to take a bufconfig.PluginConfig and turn it into a client/options pair.
type checkClientSpec struct {
	Client  check.Client
	Options check.Options
}

func newCheckClientSpec(client check.Client, options check.Options) *checkClientSpec {
	return &checkClientSpec{
		Client:  client,
		Options: options,
	}
}
