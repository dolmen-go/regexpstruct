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
	"regexp"
	"regexp/syntax"
	"slices"
	"strings"
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

type capture struct {
	Name string
	Min  int
	Max  int
	Sub  []*capture
	RE   *syntax.Regexp
}

func (c *capture) String() string {
	var s string
	if c.Name != "" {
		s = fmt.Sprintf("%q ", c.Name)
	}
	if c.Min != 1 || c.Max != 1 {
		s = s + fmt.Sprintf("{%d, %d} ", c.Min, c.Max)
	}
	const indent = "  "
	if len(c.Sub) > 0 {
		s += "[\n"
		const indent = "  "
		for _, x := range c.Sub {
			s += indent + "• " + strings.ReplaceAll(x.String(), "\n ", "\n "+indent) + "\n"
		}
		s += "]"
	}
	return s
}

func simplifyCaptureTree(subs []*capture) []*capture {
	if len(subs) == 1 && subs[0].Name == "" && len(subs[0].Sub) == 1 {
		subs[0].Sub[0].Min *= subs[0].Min
		if subs[0].Sub[0].Max == -1 || subs[0].Max == -1 {
			subs[0].Sub[0].Max = -1
		} else {
			subs[0].Sub[0].Max *= subs[0].Max
		}
		subs[0] = subs[0].Sub[0]
	}
	return subs
}

func buildCaptureTreeSub(list []*syntax.Regexp) (subs []*capture) {
	for _, sub := range list {
		tmp := buildCaptureTree(sub)
		if len(tmp) == 0 {
			continue
		}
		for _, x := range tmp {
			if x.Name == "" {
				subs = append(subs, x)
			}
			i := slices.IndexFunc(subs, func(e *capture) bool {
				return e.Name == x.Name
			})
			if i == -1 || subs[i].RE != x.RE {
				subs = append(subs, x)
			} else {
				y := subs[i]
				if x.Max == -1 || y.Max == -1 {
					y.Max = -1
				} else {
					y.Max += x.Max
				}
				y.Min += x.Min
			}
		}
	}
	return simplifyCaptureTree(subs)
}

func buildCaptureTree(re *syntax.Regexp) []*capture {
	switch re.Op {
	case syntax.OpCapture:
		var c capture
		c.Min = 1
		c.Max = 1
		c.Name = re.Name
		c.Sub = buildCaptureTreeSub(re.Sub)
		return simplifyCaptureTree([]*capture{&c})
	case syntax.OpStar:
		return simplifyCaptureTree([]*capture{{Min: 0, Max: -1, Sub: buildCaptureTreeSub(re.Sub)}})
	case syntax.OpRepeat:
		return simplifyCaptureTree([]*capture{{Min: re.Min, Max: re.Max, Sub: buildCaptureTreeSub(re.Sub)}})
	default:
		return buildCaptureTreeSub(re.Sub)
	}
}

func dumpRE(re *syntax.Regexp) (a []string) {
	nodeName := re.Op.String()
	switch re.Op {
	case syntax.OpCapture:
		nodeName += fmt.Sprintf(" #%d %q", re.Cap, re.Name)
	case syntax.OpRepeat:
		nodeName += fmt.Sprintf(" {%d, %d}", re.Min, re.Max)
	}
	a = append(a, nodeName)
	for _, sub := range re.Sub {
		for _, x := range dumpRE(sub) {
			a = append(a, "  "+x)
		}
	}
	return
}

func diagRE(t *testing.T, reStr string) {
	re, err := syntax.Parse(reStr, syntax.Perl)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("tree:\n%v", strings.Join(dumpRE(re), "\n"))
	t.Logf("tree2:\n%v", buildCaptureTree(re))
	t.Logf("SubexpNames: %#v", regexp.MustCompile(reStr).SubexpNames())

	re = re.Simplify()
	t.Log("Simplify...")
	reStrSimplify := re.String()
	t.Log(re)
	t.Logf("%#v", re)
	t.Logf("tree:\n%v", strings.Join(dumpRE(re), "\n"))
	t.Logf("tree2:\n%v", buildCaptureTree(re))
	t.Logf("SubexpNames: %#v", regexp.MustCompile(reStrSimplify).SubexpNames())
	/*
		reComp, err := syntax.Compile(re)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("Prog:\n%v", reComp)
	*/
}

func TestArray(t *testing.T) {
	diagRE(t, "(?P<char>.){3}")

	diagRE(t, "ab(?P<char>.)*cd")
}
