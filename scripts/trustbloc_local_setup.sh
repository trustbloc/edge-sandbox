#!/bin/sh
#
# Copyright SecureKey Technologies Inc. All Rights Reserved.
#
# SPDX-License-Identifier: Apache-2.0
#

SANDBOX_HOME=~/.trustbloc-local/sandbox/

mkdir -p ${SANDBOX_HOME}/certs
cp test/bdd/fixtures/keys/tls/trustbloc-dev-ca.* ${SANDBOX_HOME}/certs

# copy these entries into /etc/hosts file
cat > ${SANDBOX_HOME}/hosts <<EOF
127.0.0.1 rp.trustbloc.local
127.0.0.1 issuer.trustbloc.local
127.0.0.1 cms.trustbloc.local
127.0.0.1 myagent.trustbloc.local
127.0.0.1 router.trustbloc.local
127.0.0.1 hydra.trustbloc.local
127.0.0.1 consent-login.trustbloc.local
127.0.0.1 peer0-org1.trustbloc.local
127.0.0.1 peer1-org1.trustbloc.local
127.0.0.1 peer0-org2.trustbloc.local
127.0.0.1 peer1-org2.trustbloc.local
127.0.0.1 peer0-org3.trustbloc.local
127.0.0.1 peer1-org3.trustbloc.local
EOF
