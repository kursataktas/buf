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

package bufwkp

import (
	"context"

	"github.com/bufbuild/buf/private/bufpkg/bufprotosource"
	"github.com/bufbuild/bufplugin-go/check"
)

var (
	v2ServicePascalCaseRuleSpec = &check.RuleSpec{
		ID: "SERVICE_PASCAL_CASE",
		Categories: []string{
			"BASIC",
			"DEFAULT",
		},
		Purpose: check.NopPurpose("Checks that services are PascalCase."),
		Type:    check.RuleTypeLint,
		Handler: check.NopRuleHandler(check.RuleHandlerFunc(handleV2ServicePascalCase)),
	}
)

func handleV2ServicePascalCase(
	_ context.Context,
	responseWriter check.ResponseWriter,
	request check.Request,
) error {
	return nil
}

func newFilesRuleHandler(
	f func(check.ResponseWriter, []bufprotosource.File, check.Options) error,
) check.RuleHandler {
	return check.RuleHandlerFunc(
		func(
			_ context.Context,
			responseWriter check.ResponseWriter,
			request check.Request,
		) error {
			return nil
		},
	)
}
