# Tools
PROTOC = protoc
PROTOC_GO_PLUGIN_DIR = $(GOPATH)/bin

# Dependent repos
PROTOBUF_REPO = $(HOME)/git-repos/protobuf
GOOGLEAPIS_REPO = $(HOME)/git-repos/googleapis

GENERATOR=$(GOPATH)/bin/generate-tests

.PHONY: generate-tests sync-protos gen-protos generator

generate-tests: gen-protos sync-protos generator
	$(GENERATOR)

generator:
	go install ./cmd/generate-tests

gen-protos: sync-protos
	mkdir -p genproto
	PATH=$(PATH):$(PROTOC_GO_PLUGIN_DIR) \
		$(PROTOC) --go_out=plugins=grpc:genproto \
		-I testproto -I $(PROTOBUF_REPO)/src -I $(GOOGLEAPIS_REPO) \
		testproto/*.proto

sync-protos:
	cd $(PROTOBUF_REPO); git pull
	cd $(GOOGLEAPIS_REPO); git pull

