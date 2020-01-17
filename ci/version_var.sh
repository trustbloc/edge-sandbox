#
# Copyright SecureKey Technologies Inc. All Rights Reserved.
#
# SPDX-License-Identifier: Apache-2.0
#


# Release Parameters
BASE_VERSION=0.1.1
IS_RELEASE=false

if [ $IS_RELEASE == false ]
then
  EXTRA_VERSION=snapshot-$(git rev-parse --short=7 HEAD)
  PROJECT_VERSION=$BASE_VERSION-$EXTRA_VERSION
else
  PROJECT_VERSION=$BASE_VERSION
fi

export EDGE_SANDBOX_TAG=$PROJECT_VERSION
