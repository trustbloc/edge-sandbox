# Copyright SecureKey Technologies Inc.
#
# SPDX-License-Identifier: Apache-2.0

GO_CMD          ?= go
ARCH             = $(shell go env GOARCH)
ISSUER_REST_PATH = cmd/issuer-rest
RP_REST_PATH     = cmd/rp-rest
STRAPI_DEMO_PATH = cmd/strapi-demo
LOGIN_CONSENT_PATH = cmd/login-consent-server

DOCKER_OUTPUT_NS         ?= docker.pkg.github.com
# Namespace for the issuer image
ISSUER_REST_IMAGE_NAME   ?= trustbloc/edge-sandbox/issuer-rest
# Namespace for the rp image
RP_REST_IMAGE_NAME       ?= trustbloc/edge-sandbox/rp-rest
# Namespace for the login consent server image
LOGIN_CONSENT_SEVER_IMAGE_NAME   ?= trustbloc/edge-sandbox/login-consent-server

# Tool commands (overridable)
ALPINE_VER ?= 3.10
GO_VER     ?= 1.13.1

# Fabric tools docker image (overridable)
FABRIC_TOOLS_IMAGE   ?= hyperledger/fabric-tools
FABRIC_TOOLS_VERSION ?= 2.0.0-alpha
FABRIC_TOOLS_TAG     ?= $(ARCH)-$(FABRIC_TOOLS_VERSION)

# This can be a commit hash or a tag (or any git ref)
export FABRIC_CLI_EXT_VERSION ?= v0.1.2

.PHONY: all
all: checks unit-test

.PHONY: checks
checks: license lint

.PHONY: lint
lint:
	@scripts/check_lint.sh

.PHONY: license
license:
	@scripts/check_license.sh

.PHONY: unit-test
unit-test:
	@scripts/check_unit.sh

.PHONY: demo-start
demo-start: clean generate-test-config issuer-rest-docker rp-rest-docker login-consent-server-docker generate-test-keys
	@scripts/sandbox_start.sh

.PHONY: demo-start-with-sidetree-fabric
demo-start-with-sidetree-fabric: export START_SIDETREE_FABRIC=true
demo-start-with-sidetree-fabric: clean issuer-rest-docker rp-rest-docker login-consent-server-docker generate-test-keys populate-fixtures docker-thirdparty fabric-cli
	@scripts/sandbox_start.sh

.PHONY: demo-stop
demo-stop:
	@scripts/sandbox_stop.sh

# trustbloc-local targets enable the sandbox hosts to be accessed by friendly names.
# trustbloc-local-setup: Creates a TLS CA into ~/.trustbloc-local/sandbox/trustbloc-dev-ca.crt
#                        Creates hosts entries into ~/.trustbloc-local/sandbox/hosts.
#                        These fixtures can be manually imported into the cert chain and /etc/hosts.
.PHONY: trustbloc-local-setup
trustbloc-local-setup: trustbloc-local-remove generate-test-keys
	@scripts/trustbloc_local_setup.sh

.PHONY: trustbloc-local-remove
trustbloc-local-remove:
	rm -Rf ~/.trustbloc-local/

.PHONY: issuer-rest
issuer-rest:
	@echo "Building issuer-rest"
	@mkdir -p ./.build/bin/issuer
	@cp -r ${ISSUER_REST_PATH}/static ./.build/bin/issuer
	@cd ${ISSUER_REST_PATH} && go build -o ../../.build/bin/issuer/issuer-rest main.go

.PHONY: rp-rest
rp-rest:
	@echo "Building rp-rest"
	@mkdir -p ./.build/bin/rp
	@cp -r ${RP_REST_PATH}/static ./.build/bin/rp
	@cd ${RP_REST_PATH} && go build -o ../../.build/bin/rp/rp-rest main.go

.PHONY: login-consent-server
login-consent-server:
	@echo "Building login-consent-server"
	@mkdir -p ./.build/bin/login-consent
	@cp -r ${LOGIN_CONSENT_PATH}/templates ./.build/bin/login-consent
	@cd ${LOGIN_CONSENT_PATH} && go build -o ../../.build/bin/login-consent/server main.go

.PHONY: issuer-rest-docker
issuer-rest-docker:
	@echo "Building issuer rest docker image"
	@docker build -f ./images/issuer-rest/Dockerfile --no-cache -t $(DOCKER_OUTPUT_NS)/$(ISSUER_REST_IMAGE_NAME):latest \
	--build-arg GO_VER=$(GO_VER) \
	--build-arg ALPINE_VER=$(ALPINE_VER) .

.PHONY: rp-rest-docker
rp-rest-docker:
	@echo "Building rp rest docker image"
	@docker build -f ./images/rp-rest/Dockerfile --no-cache -t $(DOCKER_OUTPUT_NS)/$(RP_REST_IMAGE_NAME):latest \
	--build-arg GO_VER=$(GO_VER) \
	--build-arg ALPINE_VER=$(ALPINE_VER) .

.PHONY: login-consent-server-docker
login-consent-server-docker:
	@echo "Building login consent server docker image"
	@docker build -f ./images/login-consent-server/Dockerfile --no-cache -t $(DOCKER_OUTPUT_NS)/$(LOGIN_CONSENT_SEVER_IMAGE_NAME):latest \
	--build-arg GO_VER=$(GO_VER) \
	--build-arg ALPINE_VER=$(ALPINE_VER) .

.PHONY: generate-test-keys
generate-test-keys: clean
	@mkdir -p test/bdd/fixtures/keys/tls
	@cp ~/.trustbloc-local/sandbox/certs/trustbloc-dev-ca.* test/bdd/fixtures/keys/tls 2>/dev/null || :
	@docker run -i --rm \
		-v $(abspath .):/opt/workspace/edge-sandbox \
		--entrypoint "/opt/workspace/edge-sandbox/scripts/generate_test_keys.sh" \
		frapsoft/openssl


.PHONY: docker-thirdparty
docker-thirdparty:
	docker pull couchdb:2.2.0
	docker pull hyperledger/fabric-orderer:$(ARCH)-2.0.0-alpha

.PHONY: crypto-gen
crypto-gen:
	@echo "Generating crypto directory ..."
	@docker run -i \
		-v /$(abspath .):/opt/workspace/edge-sandbox -u $(shell id -u):$(shell id -g) \
		$(FABRIC_TOOLS_IMAGE):$(FABRIC_TOOLS_TAG) \
		//bin/bash -c "FABRIC_VERSION_DIR=fabric /opt/workspace/edge-sandbox/scripts/generate_crypto.sh"

.PHONY: channel-config-gen
channel-config-gen:
	@echo "Generating test channel configuration transactions and blocks ..."
	@docker run -i \
		-v /$(abspath .):/opt/workspace/edge-sandbox -u $(shell id -u):$(shell id -g) \
		$(FABRIC_TOOLS_IMAGE):$(FABRIC_TOOLS_TAG) \
		//bin/bash -c "FABRIC_VERSION_DIR=fabric/ /opt/workspace/edge-sandbox/scripts/generate_channeltx.sh"

.PHONY: populate-fixtures
populate-fixtures: clean
	@scripts/populate-fixtures.sh -f

fabric-cli:
	@scripts/build_fabric_cli.sh

create-veres-did: clean
	@mkdir -p .build
	@scripts/create_veres_did.sh

.PHONY: generate-test-config
generate-test-config: clean
	@scripts/generate_test_config.sh

.PHONY: clean
clean: clean-build

.PHONY: clean-build
clean-build:
	@rm -Rf ./.build
	@rm -Rf ./coverage.txt
	@rm -Rf ./test/bdd/fixtures/keys
	@rm -Rf ./test/bdd/fixtures/oathkeeper/rules/resource-server.json
	@rm -Rf ./test/bdd/fixtures/fabric/channel
	@rm -Rf ./test/bdd/fixtures/fabric/crypto-config
	@rm -Rf ./test/bdd/fixtures/discovery-server/config
	@rm -Rf ./test/bdd/fixtures/stakeholder-server/config
