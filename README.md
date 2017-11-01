# Cross-Language Tests for Google Firestore Clients

This repo contains:

- `proto`: the protobuffers defining the test format.

- `testdata`: the tests, as text protos.

- `cmd/generate-firestore-tests`: the Go program that generates the tests.

To regenerate the tests, you must have Go installed, and this repo must be
part of your Go workspace:
```
go get -u github.com/jba/firestore-client-tests
```
Then you can use the Makefile. It expects protoc to be on your PATH. You will
have to configure it with the locations of the protobuf and
googleapis/googleapis repos.
```
cd $GOPATH/github.com/jba/firestore-client-tests
make
```
