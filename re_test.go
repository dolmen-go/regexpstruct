// Copyright 2023 Olivier Mengué
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package regexpstruct_test

import (
	"fmt"
	"testing"

	"github.com/dolmen-go/regexpstruct"
)

func Example() {
	type pair struct {
		K string `rx:"k"`
		V string `rx:"v"`
	}

	re := regexpstruct.MustCompile[pair](`^(?P<k>.*)=(?P<v>.*)\z`, "rx")

	fmt.Printf("%#v\n", re.SubexpNames())

	var p pair
	if re.FindStringStruct("a=b", &p) {
		fmt.Printf("%#v\n", p)
	}

	// Output:
	// []string{"", "k", "v"}
	// regexpstruct_test.pair{K:"a", V:"b"}
}

func TestDeep(t *testing.T) {
	type person struct {
		Name    string `rx:"name"`
		Address struct {
			City    string `rx:"city"`
			Country string `rx:"country"`
		} `rx:"address"`
	}

	re := regexpstruct.MustCompile[person](`^(?P<name>.*) / (?P<address__city>.*) / (?P<address__country>.*)$`, "rx")
	// t.Logf("%#v", re)

	s := `Leonardo da Vinci / Florence / Italia`

	var p person
	if !re.FindStringStruct(s, &p) {
		t.Fatal("no match")
	}

	t.Logf("%#v", p)

	if p.Name != "Leonardo da Vinci" {
		t.FailNow()
	}
	if p.Address.City != "Florence" {
		t.FailNow()
	}
	if p.Address.Country != "Italia" {
		t.FailNow()
	}

	if p != re.FindAllStringStruct(s, 1)[0] {
		t.Error("mismatch between FindStringStruct and FindAllStringStruct")
	}
}

func TestEmbedded(t *testing.T) {
	type address struct {
		City    string `rx:"city"`
		Country string `rx:"country"`
	}

	type person struct {
		Name string `rx:"name"`
		address
	}

	re := regexpstruct.MustCompile[person](`^(?P<name>.*) / (?P<city>.*) / (?P<country>.*)$`, "rx")
	// t.Logf("%#v", re)

	s := `Leonardo da Vinci / Florence / Italia`

	var p person
	if !re.FindStringStruct(s, &p) {
		t.Fatal("no match")
	}

	t.Logf("%#v", p)

	if p.Name != "Leonardo da Vinci" {
		t.FailNow()
	}
	if p.City != "Florence" {
		t.FailNow()
	}
	if p.Country != "Italia" {
		t.FailNow()
	}

	if p != re.FindAllStringStruct(s, 1)[0] {
		t.Error("mismatch between FindStringStruct and FindAllStringStruct")
	}
}
