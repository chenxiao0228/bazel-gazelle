/* Copyright 2018 The Bazel Authors. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// gen_known_imports generates a .go file that with a map from either proto or
// go import strings to Bazel label strings. The imports for all languages
// are stored in a proto.csv file.

package main

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"text/template"

	"github.com/bazelbuild/bazel-gazelle/internal/label"
)

var progName = filepath.Base(os.Args[0])

var knownImportsTpl = template.Must(template.New("known_imports.go").Parse(`
// Generated by internal/language/proto/gen/gen_known_imports.go
// From {{.ProtoCsv}}

package {{.Package}}

import "github.com/bazelbuild/bazel-gazelle/internal/label"

var {{.Var}} = map[string]label.Label{
{{range .Bindings}}
	{{printf "%q: label.New(%q, %q, %q)" .Import .Label.Repo .Label.Pkg .Label.Name}},
{{- end}}
}
`))

type data struct {
	ProtoCsv, Package, Var string
	Bindings               []binding
}

type binding struct {
	Import string
	Label  label.Label
}

func main() {
	log.SetPrefix(progName + ": ")
	log.SetFlags(0)
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(args []string) (err error) {
	fs := flag.NewFlagSet(progName, flag.ExitOnError)
	var protoCsvPath, knownImportsPath, package_, var_ string
	var keyColumn, valueColumn int
	fs.StringVar(&protoCsvPath, "proto_csv", "", "path to proto.csv input file")
	fs.StringVar(&knownImportsPath, "known_imports", "", "path to known_imports.go output file")
	fs.StringVar(&package_, "package", "", "package name in generated file")
	fs.StringVar(&var_, "var", "", "var name in generated file")
	fs.IntVar(&keyColumn, "key", 0, "key column number")
	fs.IntVar(&valueColumn, "value", 1, "value column number")
	fs.Parse(args)
	if protoCsvPath == "" {
		return fmt.Errorf("-proto_csv not set")
	}
	if knownImportsPath == "" {
		return fmt.Errorf("-known_imports not set")
	}
	if package_ == "" {
		return fmt.Errorf("-package not set")
	}
	if var_ == "" {
		return fmt.Errorf("-var not set")
	}

	protoCsvFile, err := os.Open(protoCsvPath)
	if err != nil {
		return err
	}
	r := csv.NewReader(bufio.NewReader(protoCsvFile))
	r.Comment = '#'
	r.FieldsPerRecord = 4
	records, err := r.ReadAll()
	protoCsvFile.Close()
	if err != nil {
		return err
	}
	data := data{
		ProtoCsv: protoCsvPath,
		Package:  package_,
		Var:      var_,
	}
	seen := make(map[string]label.Label)
	for _, rec := range records {
		imp := rec[keyColumn]
		lbl, err := label.Parse(rec[valueColumn])
		if err != nil {
			return err
		}
		if seenLabel, ok := seen[imp]; ok {
			if !seenLabel.Equal(lbl) {
				return fmt.Errorf("for key %s, multiple values (%s and %s)", imp, seenLabel, lbl)
			}
			continue
		}
		seen[imp] = lbl
		data.Bindings = append(data.Bindings, binding{imp, lbl})
	}

	knownImportsBuf := &bytes.Buffer{}
	if err := knownImportsTpl.Execute(knownImportsBuf, data); err != nil {
		return err
	}
	return ioutil.WriteFile(knownImportsPath, knownImportsBuf.Bytes(), 0666)
}
