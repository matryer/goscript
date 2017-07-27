package goscript

import (
	"fmt"
	"testing"

	"github.com/matryer/is"
)

func TestGoscript(t *testing.T) {
	is := is.New(t)
	script := New(`
func goscript(salutation, name string) (string, error) {
	greeting := salutation + " " + name
	return greeting, nil
}
`)
	defer script.Close()
	greeting, err := script.Execute("Hello", "Mat")
	is.NoErr(err) // Execute
	is.Equal(greeting, "Hello Mat")
}

var tests = []struct {
	Goscript string
	InArgs   []interface{}
	OutValue interface{}
	OutErr   string
}{
	{
		Goscript: `
func goscript() (string, error) {
	return "Hello", nil
}
`,
		OutValue: "Hello",
	},
	{
		Goscript: `
func goscript(salutation, name string) (string, error) {
	greeting := salutation + " " + name
	return greeting, nil
}
`,
		InArgs:   []interface{}{"Hello", "Mat"},
		OutValue: "Hello Mat",
	},
	{
		Goscript: `
import "strings"

func goscript(items ...string) (string, error) {
	return strings.Join(items, ","), nil
}
`,
		InArgs:   []interface{}{"one", "two", "three"},
		OutValue: "one,two,three",
	},
	{
		Goscript: `
import "strings"

func goscript(separator string, items ...string) (string, error) {
	return strings.Join(items, separator), nil
}
`,
		InArgs:   []interface{}{"|", "one", "two", "three"},
		OutValue: "one|two|three",
	},
	{
		Goscript: `
import "strings"

func goscript(separator string, items) (string, error) {
	return strings.Join(items, separator), nil
}
`,
		InArgs: []interface{}{"|", "one", "two", "three"},
		OutErr: "goscript:4: syntax error: mixed named and unnamed function parameters",
	},
}

func TestGoscriptTests(t *testing.T) {
	is := is.New(t)
	for i := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			is := is.New(t)
			test := tests[i]
			script := New(test.Goscript)
			defer script.Close()
			val, err := script.Execute(test.InArgs...)
			if err != nil {
				is.Equal(err.Error(), test.OutErr)
			} else {
				if test.OutErr != "" {
					is.Fail() // OutErr specified but error was nil
				}
			}
			if test.OutValue != nil {
				is.Equal(val, test.OutValue)
			}
		})
	}
}

func TestExtractArguments(t *testing.T) {
	is := is.New(t)

	in := extractArguments(`func goscript(one, two, three string, age int) (interface{}, error)`)

	is.Equal(len(in), 4)
	is.Equal(in[0].Index, 0)
	is.Equal(in[0].Name, "one")
	is.Equal(in[0].Typ, "string")
	is.Equal(in[1].Index, 1)
	is.Equal(in[1].Name, "two")
	is.Equal(in[1].Typ, "string")
	is.Equal(in[2].Index, 2)
	is.Equal(in[2].Name, "three")
	is.Equal(in[2].Typ, "string")
	is.Equal(in[3].Index, 3)
	is.Equal(in[3].Name, "age")
	is.Equal(in[3].Typ, "int")

	in = extractArguments(`func goscript(args ...interface{}) (interface{}, error)`)
	is.Equal(len(in), 1)
	is.Equal(in[0].Index, 0)
	is.Equal(in[0].Name, "args")
	is.Equal(in[0].Typ, "...interface{}")

	in = extractArguments(`func goscript() (interface{}, error)`)
	is.Equal(len(in), 0)

}
