// Copyright 2020-2022 Buf Technologies, Inc.
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

// Code generated by protoc-gen-go-apiclientconnect. DO NOT EDIT.

package registryv1alpha1apiclientconnect

import (
	context "context"
	registryv1alpha1connect "github.com/bufbuild/buf/private/gen/proto/connect/buf/alpha/registry/v1alpha1/registryv1alpha1connect"
	v1alpha1 "github.com/bufbuild/buf/private/gen/proto/go/buf/alpha/registry/v1alpha1"
	connect_go "github.com/bufbuild/connect-go"
	zap "go.uber.org/zap"
)

type referenceServiceClient struct {
	logger *zap.Logger
	client registryv1alpha1connect.ReferenceServiceClient
}

func (s *referenceServiceClient) Unwrap() registryv1alpha1connect.ReferenceServiceClient {
	return s.client
}

// GetReferenceByName takes a reference name and returns the
// reference either as 'main', a tag, or commit.
func (s *referenceServiceClient) GetReferenceByName(
	ctx context.Context,
	name string,
	owner string,
	repositoryName string,
) (reference *v1alpha1.Reference, _ error) {
	response, err := s.client.GetReferenceByName(
		ctx,
		connect_go.NewRequest(
			&v1alpha1.GetReferenceByNameRequest{
				Name:           name,
				Owner:          owner,
				RepositoryName: repositoryName,
			}),
	)
	if err != nil {
		return nil, err
	}
	return response.Msg.Reference, nil
}
