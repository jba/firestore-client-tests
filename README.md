# Cross-Language Tests for Google Firestore Clients

This repo contains:

- `proto`: the protobuffers defining the test format.

- `testdata`: the tests.
   - `*.textproto`: a single test in text proto format.
   - `tests.binprotos`: all the tests in a single file. The protos are in binary
      format, each preceded by its length encoded as a varint.

Do not modify the tests here. Regenerate them using the Go program at
`cloud.google.com/go/firestore/cmd/generate-firestore-tests`.
