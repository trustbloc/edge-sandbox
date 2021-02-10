/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package operation

import "encoding/json"

type createVaultResp struct {
	ID string `json:"id"`
}

type saveDocReq struct {
	ID      string          `json:"id"`
	Content json.RawMessage `json:"content"`
	Tags    []string        `json:"tags"`
}