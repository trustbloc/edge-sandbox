// Copyright SecureKey Technologies Inc. All Rights Reserved.
//
// SPDX-License-Identifier: Apache-2.0

module github.com/trustbloc/edge-sandbox/cmd/issuer-rest

replace github.com/trustbloc/edge-sandbox => ../..

require (
	github.com/gorilla/mux v1.7.3
	github.com/spf13/cobra v0.0.5
	github.com/stretchr/testify v1.4.0
	github.com/trustbloc/edge-sandbox v0.0.0
	golang.org/x/oauth2 v0.0.0-20191202225959-858c2ad4c8b6
)

go 1.13
