package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	tpb "./genproto"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	fspb "google.golang.org/genproto/googleapis/firestore/v1beta1"
)

const (
	database = "projects/projectID/databases/(default)"
	docPath  = database + "/documents/C/d"
)

var (
	updateTimePrecondition *fspb.Precondition
	existsTruePrecondition = &fspb.Precondition{
		ConditionType: &fspb.Precondition_Exists{true},
	}

	nTests    int
	outputDir string
)

func main() {
	outputDir = filepath.Join(os.Getenv("GOPATH"),
		"src/cloud.google.com/go/firestore/testdata")

	aTime := time.Date(2017, 1, 2, 3, 4, 5, 6, time.UTC)
	aTimestamp, err := ptypes.TimestampProto(aTime)
	if err != nil {
		log.Fatal(err)
	}
	updateTimePrecondition = &fspb.Precondition{
		ConditionType: &fspb.Precondition_UpdateTime{aTimestamp},
	}

	genGet()
	genCreate()
	genSet()
	genUpdate()
	genUpdatePaths()
	genDelete()
	fmt.Printf("wrote %d tests to %s\n", nTests, outputDir)
}

func genGet() {
	writeTest("get-1", &tpb.Test{
		Description: "Get a document",
		Test: &tpb.Test_Get{&tpb.GetTest{
			DocRefPath: docPath,
			Request:    &fspb.GetDocumentRequest{Name: docPath},
		}},
	})
}

func genCreate() {
	for i, test := range []struct {
		desc      string
		data      string
		write     map[string]*fspb.Value
		mask      []string
		transform []string
		isErr     bool
	}{
		{
			desc:  "basic create",
			data:  `{"a": 1}`,
			write: mp("a", 1),
		},
		{
			desc:  "don’t split on dots", // go/set-update #1
			data:  `{ "a.b": { "c.d": 1 }, "e": 2 }`,
			write: mp("a.b", mp("c.d", 1), "e", 2),
		},
		{
			desc:      "a ServerTimestamp field becomes a transform",
			data:      `{"a": 1, "b": "ServerTimestamp"}`,
			write:     mp("a", 1),
			transform: []string{"b"},
		},
		{
			desc:      "nested ServerTimestamp field",
			data:      `{"a": 1, "b": {"c": "ServerTimestamp"}}`,
			write:     mp("a", 1),
			transform: []string{"b.c"},
		},
		{
			desc:      "multiple ServerTimestamp fields",
			data:      `{"a": 1, "b": "ServerTimestamp", "c": {"d": "ServerTimestamp"}}`,
			write:     mp("a", 1),
			transform: []string{"b", "c.d"},
		},
		// Errors:
		{
			desc:  "ServerTimestamp cannot be in an array value",
			data:  `{"a": ["ServerTimestamp"]}`,
			isErr: true,
		},
		{
			desc:  "Delete cannot appear in data",
			data:  `{"a": 1, "b": "Delete"}`,
			isErr: true,
		},
	} {
		var req *fspb.CommitRequest
		if !test.isErr {
			req = newCommitRequest(test.write, test.mask, test.transform)
			req.Writes[0].CurrentDocument = &fspb.Precondition{
				ConditionType: &fspb.Precondition_Exists{false},
			}
		}
		tp := &tpb.Test{
			Description: test.desc,
			Test: &tpb.Test_Create{&tpb.CreateTest{
				DocRefPath: docPath,
				JsonData:   test.data,
				Request:    req,
				IsError:    test.isErr,
			}},
		}
		writeTest(fmt.Sprintf("create-%d", i+1), tp)
	}

}
func genSet() {
	for i, test := range []struct {
		desc      string
		data      string
		opt       *tpb.SetOption
		write     map[string]*fspb.Value
		mask      []string
		transform []string
		isErr     bool
	}{
		{
			desc:  "Set with no options",
			data:  `{"a": 1}`,
			write: mp("a", 1),
		},
		{
			desc:  "Don't split on dots", // go/set-update #2
			data:  `{ "a.b": { "f.g": 2 }, "h": { "g": 3 } }`,
			write: mp("a.b", mp("f.g", 2), "h", mp("g", 3)),
		},
		{
			desc:  "MergeAll",
			data:  `{"a": 1, "b": 2}`,
			opt:   mergeAllOption,
			write: mp("a", 1, "b", 2),
			mask:  []string{"a", "b"},
		},
		{
			desc:  "MergeAll with nested fields", // go/set-update #3
			data:  `{"h": { "g": 3, "f": 4 }}`,
			opt:   mergeAllOption,
			write: mp("h", mp("g", 3, "f", 4)),
			mask:  []string{"h.f", "h.g"},
		},
		{
			desc:  "Merge with a field",
			data:  `{"a": 1, "b": 2}`,
			opt:   mergeOption("a"),
			write: mp("a", 1),
			mask:  []string{"a"},
		},
		{
			desc:  "Merge with a nested field", // go/set-update #4
			data:  `{"h": {"g": 4, "f": 5}}`,
			opt:   mergeOption("h.g"),
			write: mp("h", mp("g", 4)),
			mask:  []string{"h.g"},
		},
		{
			desc:  "Merge field is not a leaf", // go/set-update #5
			data:  `{"h": {"g": 5, "f": 6}, "e": 7}`,
			opt:   mergeOption("h"),
			write: mp("h", mp("g", 5, "f", 6)),
			mask:  []string{"h"},
		},
		{
			desc:  "Merge with FieldPaths",
			data:  `{"*": {"~": true}}`,
			opt:   mergePathsOption([]string{"*", "~"}),
			write: mp("*", mp("~", true)),
			mask:  []string{"`*`.`~`"},
		},
		{
			desc:      "a ServerTimestamp field becomes a transform",
			data:      `{"a": 1, "b": "ServerTimestamp"}`,
			write:     mp("a", 1),
			transform: []string{"b"},
		},
		{
			desc:      "nested ServerTimestamp field",
			data:      `{"a": 1, "b": {"c": "ServerTimestamp"}}`,
			write:     mp("a", 1),
			transform: []string{"b.c"},
		},
		{
			desc:      "multiple ServerTimestamp fields",
			data:      `{"a": 1, "b": "ServerTimestamp", "c": {"d": "ServerTimestamp"}}`,
			write:     mp("a", 1),
			transform: []string{"b", "c.d"},
		},
		{
			desc:      "ServerTimestamp with MergeAll",
			data:      `{"a": 1, "b": "ServerTimestamp"}`,
			opt:       mergeAllOption,
			write:     mp("a", 1),
			mask:      []string{"a"},
			transform: []string{"b"},
		},
		{
			desc:      "ServerTimestamp with Merge of both fields",
			data:      `{"a": 1, "b": "ServerTimestamp"}`,
			opt:       mergeOption("a", "b"),
			write:     mp("a", 1),
			mask:      []string{"a"},
			transform: []string{"b"},
		},
		{
			desc:  "If is ServerTimestamp not in Merge, no transform",
			data:  `{"a": 1, "b": "ServerTimestamp"}`,
			opt:   mergeOption("a"),
			write: mp("a", 1),
			mask:  []string{"a"},
		},
		{
			desc:      "If no ordinary values in Merge, no write",
			data:      `{"a": 1, "b": "ServerTimestamp"}`,
			opt:       mergeOption("b"),
			transform: []string{"b"},
		},
		// Errors:
		{
			desc:  "Merge fields must all be present in data",
			data:  `{"a": 1}`,
			opt:   mergeOption("b", "a"),
			isErr: true,
		},
		{
			desc:  "ServerTimestamp cannot be in an array value",
			data:  `{"a": ["ServerTimestamp"]}`,
			isErr: true,
		},
		{
			desc:  "Delete cannot appear in data",
			data:  `{"a": 1, "b": "Delete"}`,
			isErr: true,
		},
		{
			desc:  "Delete cannot even appear in an unmerged field",
			data:  `{"a": 1, "b": "Delete"}`,
			opt:   mergeOption("a"),
			isErr: true,
		},
	} {
		var opts []*tpb.SetOption
		if test.opt != nil {
			opts = []*tpb.SetOption{test.opt}
		}
		var req *fspb.CommitRequest
		if !test.isErr {
			req = newCommitRequest(test.write, test.mask, test.transform)
		}
		tp := &tpb.Test{
			Description: test.desc,
			Test: &tpb.Test_Set{&tpb.SetTest{
				DocRefPath: docPath,
				Options:    opts,
				JsonData:   test.data,
				Request:    req,
				IsError:    test.isErr,
			}},
		}
		writeTest(fmt.Sprintf("set-%d", i+1), tp)
	}
}

func genUpdate() {
	for i, test := range []struct {
		desc      string
		data      string
		precond   *fspb.Precondition
		write     map[string]*fspb.Value
		mask      []string
		transform []string
		isErr     bool
	}{
		{
			desc:  "basic update",
			data:  `{"a": 1, "b": 2}`,
			write: mp("a", 1, "b", 2),
			mask:  []string{"a", "b"},
		},
		{
			desc:  "nested paths",
			data:  `{"a": 1, "b": {"c": 2}}`,
			write: mp("a", 1, "b", mp("c", 2)),
			mask:  []string{"a", "b"},
		},
		{
			desc:  "split on dots",
			data:  `{"a.b.c": 1}`,
			write: mp("a", mp("b", mp("c", 1))),
			mask:  []string{"a.b.c"},
		},
		{
			desc:  "Split on dots for top-level keys only", // go/set-update #6
			data:  `{"h.g": {"j.k": 6}}`,
			write: mp("h", mp("g", mp("j.k", 6))),
			mask:  []string{"h.g"},
		},
		{
			desc:  "Delete",
			data:  `{"a": 1, "b": "Delete"}`,
			write: mp("a", 1),
			mask:  []string{"a", "b"},
		},
		{
			desc:  "Delete alone",
			data:  `{"a": "Delete"}`,
			write: mp(),
			mask:  []string{"a"},
		},
		{
			desc:  "Delete with a dotted field",
			data:  `{"a": 1, "b.c": "Delete"}`,
			write: mp("a", 1),
			mask:  []string{"a", "b.c"},
		},
		{
			desc:    "last-update-time precondition",
			data:    `{"a": 1}`,
			precond: updateTimePrecondition,
			write:   mp("a", 1),
			mask:    []string{"a"},
		},
		{
			desc:      "a ServerTimestamp field becomes a transform",
			data:      `{"a": 1, "b": "ServerTimestamp"}`,
			write:     mp("a", 1),
			mask:      []string{"a"},
			transform: []string{"b"},
		},
		{
			desc:      "nested ServerTimestamp field",
			data:      `{"a": 1, "b": {"c": "ServerTimestamp"}}`,
			write:     mp("a", 1),
			mask:      []string{"a", "b"},
			transform: []string{"b.c"},
		},
		{
			desc:  "multiple ServerTimestamp fields",
			data:  `{"a": 1, "b": "ServerTimestamp", "c": {"d": "ServerTimestamp"}}`,
			write: mp("a", 1),
			// b is not in the mask because it will be set in the transform.
			// c must be in the mask: it should be replaced entirely. The transform
			// will set c.d to the timestamp, but the update will delete the rest of c.
			mask:      []string{"a", "c"},
			transform: []string{"b", "c.d"},
		},
		{
			desc: "ServerTimestamp with dotted field",
			data: `{"a.b.c": "ServerTimestamp"}`,
			// We need an update to carry the precondition.
			mask:      []string{},
			transform: []string{"a.b.c"},
		},
		// Errors
		{
			desc:  "no paths",
			data:  `{}`,
			isErr: true,
		},
		{
			desc:  "invalid character",
			data:  `{"a~b": 1}`,
			isErr: true,
		},
		{
			desc:  "a path component cannot be empty",
			data:  `{"a..b": 1}`,
			isErr: true,
		},
		{
			desc:  "one field cannot be a prefix of another",
			data:  `{"a.b": 1, "a": 2}`,
			isErr: true,
		},
		{
			desc:  "Delete cannot be in an array value",
			data:  `{"a.b": 1, "a": [2, 3, "Delete"]}`,
			isErr: true,
		},
		{
			desc:  "Delete cannot be nested",
			data:  `{"a": {"b": "Delete"}}`,
			isErr: true,
		},
		{
			desc:    "Exists precondition is invalid",
			data:    `{"a": 1}`,
			precond: existsTruePrecondition,
			isErr:   true,
		},
	} {
		var req *fspb.CommitRequest
		if !test.isErr {
			req = newCommitRequest(test.write, test.mask, test.transform)
			if test.precond != nil {
				req.Writes[0].CurrentDocument = test.precond
			} else {
				req.Writes[0].CurrentDocument = existsTruePrecondition
			}
		}
		tp := &tpb.Test{
			Description: test.desc,
			Test: &tpb.Test_Update{&tpb.UpdateTest{
				DocRefPath:   docPath,
				Precondition: test.precond,
				JsonData:     test.data,
				Request:      req,
				IsError:      test.isErr,
			}},
		}
		writeTest(fmt.Sprintf("update-%d", i+1), tp)
	}
}

func genUpdatePaths() {
	for i, test := range []struct {
		desc      string
		paths     [][]string
		values    []string
		precond   *fspb.Precondition
		write     map[string]*fspb.Value
		mask      []string
		transform []string
		isErr     bool
	}{
		{
			desc:   "basic call",
			paths:  [][]string{{"a"}, {"b", "c"}},
			values: []string{`1`, `2`},
			write:  mp("a", 1, "b", mp("c", 2)),
			mask:   []string{"a", "b.c"},
		},
		{
			desc:   "FieldPath elements are not split on dots", // go/set-update #7, approx.
			paths:  [][]string{{"a.b", "f.g"}},
			values: []string{`{"n.o": 7}`},
			write:  mp("a.b", mp("f.g", mp("n.o", 7))),
			mask:   []string{"`a.b`.`f.g`"},
		},
		{
			desc:   "special characters",
			paths:  [][]string{{"*", "~"}, {"*", "/"}},
			values: []string{`1`, `2`},
			write:  mp("*", mp("~", 1, "/", 2)),
			mask:   []string{"`*`.`/`", "`*`.`~`"},
		},
		{
			desc:    "last-update-time precondition",
			paths:   [][]string{{"a"}},
			values:  []string{`1`},
			precond: updateTimePrecondition,
			write:   mp("a", 1),
			mask:    []string{"a"},
		},
		{
			desc:   "Delete",
			paths:  [][]string{{"a", "b"}, {"b", "c"}},
			values: []string{`1`, `"Delete"`},
			write:  mp("a", mp("b", 1)),
			mask:   []string{"a.b", "b.c"},
		},
		{
			desc:   "Delete alone",
			paths:  [][]string{{"a", "b"}},
			values: []string{`"Delete"`},
			write:  mp(),
			mask:   []string{"a.b"},
		},
		{
			desc:      "ServerTimestamp",
			paths:     [][]string{{"a", "b"}, {"c"}},
			values:    []string{`"ServerTimestamp"`, `1`},
			write:     mp("c", 1),
			mask:      []string{"c"},
			transform: []string{"a.b"},
		},
		{
			desc:      "ServerTimestamp alone",
			paths:     [][]string{{"a", "b"}},
			values:    []string{`"ServerTimestamp"`},
			write:     nil,
			mask:      []string{},
			transform: []string{"a.b"},
		},
		// Errors
		{
			desc:   "no updates",
			paths:  nil,
			values: nil,
			isErr:  true,
		},
		{
			desc:   "empty field path",
			paths:  [][]string{{}},
			values: []string{`1`},
			isErr:  true,
		},
		{
			desc:   "empty field path component",
			paths:  [][]string{{"*", ""}},
			values: []string{`1`},
			isErr:  true,
		},
		{
			desc:   "the same field cannot occur more than once",
			paths:  [][]string{{"a"}, {"b"}, {"a"}},
			values: []string{`1`, `2`, `3`},
			isErr:  true,
		},
		{
			desc:   "one field cannot be a prefix of another",
			paths:  [][]string{{"*", "a"}, {"b"}, {"*", "a", "b"}},
			values: []string{`1`, `2`, `3`},
			isErr:  true,
		},
		{
			desc:    "Exists precondition is invalid",
			paths:   [][]string{{"a"}},
			values:  []string{`1`},
			precond: existsTruePrecondition,
			isErr:   true,
		},
		{
			desc:   "Delete cannot be in an array value",
			paths:  [][]string{{"a"}},
			values: []string{`["Delete"]`},
			isErr:  true,
		},
		{
			desc:   "Delete cannot be nested",
			paths:  [][]string{{"a"}},
			values: []string{`{"b": "Delete"}`},
			isErr:  true,
		},
	} {
		var req *fspb.CommitRequest
		if !test.isErr {
			req = newCommitRequest(test.write, test.mask, test.transform)
			if test.precond != nil {
				req.Writes[0].CurrentDocument = test.precond
			} else {
				req.Writes[0].CurrentDocument = existsTruePrecondition
			}
		}
		tp := &tpb.Test{
			Description: test.desc,
			Test: &tpb.Test_UpdatePaths{&tpb.UpdatePathsTest{
				DocRefPath:   docPath,
				Precondition: test.precond,
				FieldPaths:   toFieldPaths(test.paths),
				JsonValues:   test.values,
				Request:      req,
				IsError:      test.isErr,
			}},
		}
		writeTest(fmt.Sprintf("update-paths-%d", i+1), tp)
	}
}

func genDelete() {
	for i, test := range []struct {
		desc    string
		precond *fspb.Precondition
		isErr   bool
	}{
		{
			desc:    "delete without precondition",
			precond: nil,
		},
		{
			desc:    "delete with last-update-time precondition",
			precond: updateTimePrecondition,
		},
		{
			desc:    "delete with exists precondition",
			precond: existsTruePrecondition,
		},
	} {
		var req *fspb.CommitRequest
		if !test.isErr {
			req = &fspb.CommitRequest{
				Database: database,
				Writes:   []*fspb.Write{{Operation: &fspb.Write_Delete{docPath}}},
			}
			if test.precond != nil {
				req.Writes[0].CurrentDocument = test.precond
			}
		}
		tp := &tpb.Test{
			Description: test.desc,
			Test: &tpb.Test_Delete{&tpb.DeleteTest{
				DocRefPath:   docPath,
				Precondition: test.precond,
				Request:      req,
				IsError:      test.isErr,
			}},
		}
		writeTest(fmt.Sprintf("delete-%d", i+1), tp)
	}
}

func newCommitRequest(writeFields map[string]*fspb.Value, mask, transform []string) *fspb.CommitRequest {
	var writes []*fspb.Write
	if writeFields != nil || mask != nil {
		w := &fspb.Write{}
		if writeFields != nil || mask != nil {
			w.Operation = &fspb.Write_Update{
				Update: &fspb.Document{
					Name:   docPath,
					Fields: writeFields,
				},
			}
		}
		if mask != nil {
			w.UpdateMask = &fspb.DocumentMask{FieldPaths: mask}
		}
		writes = append(writes, w)
	}
	if transform != nil {
		var fts []*fspb.DocumentTransform_FieldTransform
		for _, p := range transform {
			fts = append(fts, &fspb.DocumentTransform_FieldTransform{
				FieldPath: p,
				TransformType: &fspb.DocumentTransform_FieldTransform_SetToServerValue{
					fspb.DocumentTransform_FieldTransform_REQUEST_TIME,
				},
			})
		}
		writes = append(writes, &fspb.Write{
			Operation: &fspb.Write_Transform{
				&fspb.DocumentTransform{
					Document:        docPath,
					FieldTransforms: fts,
				},
			},
		})
	}
	return &fspb.CommitRequest{
		Database: database,
		Writes:   writes,
	}
}

var mergeAllOption = &tpb.SetOption{All: true}

func mergeOption(paths ...string) *tpb.SetOption {
	var fps [][]string
	for _, p := range paths {
		fps = append(fps, strings.Split(p, "."))
	}
	return mergePathsOption(fps...)
}

func mergePathsOption(fps ...[]string) *tpb.SetOption {
	return &tpb.SetOption{Fields: toFieldPaths(fps)}
}

func toFieldPaths(fps [][]string) []*tpb.FieldPath {
	var ps []*tpb.FieldPath
	for _, fp := range fps {
		ps = append(ps, &tpb.FieldPath{fp})
	}
	return ps
}

func writeTest(filename string, t *tpb.Test) {
	pathname := filepath.Join(outputDir, fmt.Sprintf("%s.textproto", filename))
	if err := writeTestToFile(pathname, t); err != nil {
		log.Fatalf("writing test: %v", err)
	}
	nTests++
}

func writeTestToFile(pathname string, t *tpb.Test) error {
	f, err := os.Create(pathname)
	if err != nil {
		return err
	}
	if err := proto.MarshalText(f, t); err != nil {
		return err
	}
	return f.Close()
}

func mp(args ...interface{}) map[string]*fspb.Value {
	m := map[string]*fspb.Value{}
	for i := 0; i < len(args); i += 2 {
		m[args[i].(string)] = val(args[i+1])
	}
	return m
}

func val(a interface{}) *fspb.Value {
	switch x := a.(type) {
	case int:
		return intval(x)
	case bool:
		return boolval(x)
	case map[string]*fspb.Value:
		return mapval(x)
	default:
		log.Fatalf("val: bad type: %T", a)
		return nil
	}
}

func intval(i int) *fspb.Value {
	return &fspb.Value{&fspb.Value_IntegerValue{int64(i)}}
}

func boolval(b bool) *fspb.Value {
	return &fspb.Value{&fspb.Value_BooleanValue{b}}
}

func mapval(m map[string]*fspb.Value) *fspb.Value {
	return &fspb.Value{&fspb.Value_MapValue{&fspb.MapValue{m}}}
}
