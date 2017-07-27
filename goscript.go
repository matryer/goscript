package goscript

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"text/template"
)

// Error represents a Goscript error.
type Error struct {
	Err    error
	Stderr string
}

func (e Error) Error() string {
	return fmt.Sprintf("%s", e.Stderr)
}

// Script represents a script.
type Script struct {
	err         error
	scriptFile  string
	scriptLines int
	cmd         *exec.Cmd

	executeLock sync.Mutex

	stdin         io.WriteCloser
	stdinencoder  *gob.Encoder
	stdout        io.ReadCloser
	stdoutdecoder *gob.Decoder
	stderr        io.ReadCloser
}

// New makes a new running Script.
// Caller must call Close.
func New(script string) *Script {
	s := &Script{
		err: scriptHarnessTemplateErr,
	}
	if s.err != nil {
		return s
	}
	var args []arg
	if s.scriptLines, args, s.err = processScript(script); s.err != nil {
		return s
	}
	if s.scriptFile, s.err = createScriptFile(script, args); s.err != nil {
		return s
	}
	s.cmd = exec.Command("go", "run", s.scriptFile)
	if s.stdin, s.err = s.cmd.StdinPipe(); s.err != nil {
		return s
	}
	s.stdinencoder = gob.NewEncoder(s.stdin)
	if s.stdout, s.err = s.cmd.StdoutPipe(); s.err != nil {
		return s
	}
	s.stdoutdecoder = gob.NewDecoder(s.stdout)
	if s.stderr, s.err = s.cmd.StderrPipe(); s.err != nil {
		return s
	}
	if s.err = s.cmd.Start(); s.err != nil {
		return s
	}
	var state string
	if err := s.stdoutdecoder.Decode(&state); err != nil {
		b, _ := ioutil.ReadAll(s.stderr)
		output := processOutput(s.scriptLines, b)
		if s.err = s.cmd.Wait(); s.err != nil {
			s.err = Error{Err: s.err, Stderr: output}
		}
		return s
	}
	if state != "ready" {
		s.err = errors.New("goscript failed to start")
	}
	return s
}

// Execute executes the script with the specified arguments, and
// returns the response.
func (s *Script) Execute(args ...interface{}) (interface{}, error) {
	if s.err != nil {
		return nil, s.err
	}
	if len(args) == 0 {
		args = []interface{}{}
	}
	s.executeLock.Lock()
	defer s.executeLock.Unlock()
	// send request
	if err := s.stdinencoder.Encode(args); err != nil {
		return nil, s.cmdErr(err)
	}
	// handle response
	var res response
	if err := s.stdoutdecoder.Decode(&res); err != nil {
		return nil, s.cmdErr(err)
	}
	return res.Value, res.Error
}

func processScript(script string) (int, []arg, error) {
	n := 0
	s := bufio.NewScanner(strings.NewReader(script))
	for s.Scan() {
		n++
		trimline := strings.TrimSpace(s.Text())
		if !strings.HasPrefix(trimline, "func goscript(") {
			continue
		}
		args := extractArguments(trimline)
		return n, args, nil
	}
	return 0, nil, errors.New("missing func goscript")
}

type arg struct {
	Index int
	Name  string
	Typ   string
}

func (a arg) Variadic() bool {
	return strings.HasPrefix(a.Typ, "...")
}

func (a arg) Argname() string {
	if a.Variadic() {
		return a.Name + "..."
	}
	return a.Name
}

func (a arg) Typename() string {
	if a.Variadic() {
		return "[]" + a.Typ[3:]
	}
	return a.Typ
}

func (a arg) TypenameSingular() string {
	if a.Variadic() {
		return a.Typ[3:]
	}
	return a.Typ
}

func extractArguments(code string) []arg {
	segs := strings.Split(code, "(")
	segs = strings.Split(segs[1], ")")
	segs = strings.Split(segs[0], ",")
	if segs[0] == "" {
		return nil
	}
	args := make([]arg, len(segs))
	for i := range segs {
		var name, typ string
		ss := strings.Split(strings.TrimSpace(segs[i]), " ")
		name = ss[0]
		if len(ss) > 1 {
			typ = ss[1]
			// go back and fill in any missing types
			for j := i - 1; j >= 0; j-- {
				if args[j].Typ != "" {
					break
				}
				args[j].Typ = typ
			}
		}
		args[i] = arg{
			Index: i,
			Name:  name,
			Typ:   typ,
		}
	}
	return args
}

func createScriptFile(script string, args []arg) (string, error) {
	dir, err := ioutil.TempDir("", "goscript")
	if err != nil {
		return "", err
	}
	f, err := os.Create(filepath.Join(dir, "goscript.go"))
	if err != nil {
		return "", err
	}
	argnames := make([]string, len(args))
	for i := range args {
		argnames[i] = args[i].Argname()
	}
	data := struct {
		Goscript string
		InArgs   []arg
		ArgsList string
	}{
		Goscript: script,
		InArgs:   args,
		ArgsList: strings.Join(argnames, ", "),
	}
	if err := scriptHarnessTemplate.Execute(f, data); err != nil {
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	// to debug generated code, uncomment this block
	// if err := scriptHarnessTemplate.Execute(os.Stdout, data); err != nil {
	// 	return "", err
	// }
	return f.Name(), nil
}

func (s *Script) cmdErr(err error) error {
	if s.cmd.ProcessState != nil && s.cmd.ProcessState.Exited() {
		if !s.cmd.ProcessState.Success() {
			stderrb, _ := ioutil.ReadAll(s.stderr)
			return errors.New(string(stderrb))
		}
	}
	return err
}

// Close shuts down the script and cleans up any used resources.
func (s *Script) Close() error {
	defer os.Remove(s.scriptFile)
	if s.stdin != nil {
		s.stdin.Close()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		if s.cmd.ProcessState == nil || !s.cmd.ProcessState.Exited() {
			s.cmd.Process.Kill()
		}
	}
	if s.stdout != nil {
		s.stdout.Close()
	}
	if s.stderr != nil {
		s.stderr.Close()
	}
	return nil
}

func processOutput(scriptLines int, out []byte) string {
	var lines []string
	s := bufio.NewScanner(bytes.NewReader(out))
	for s.Scan() {
		line := s.Text()
		trimline := strings.TrimSpace(line)
		if trimline == "# command-line-arguments" {
			continue
		}
		if strings.Contains(line, "goscript.go:") {
			// error lines should be tweaked
			segs := strings.Split(line, ":")
			segs[0] = "goscript"
			n, err := strconv.Atoi(segs[1])
			if err == nil {
				scriptlineN := n - scriptStartLine
				if scriptlineN > scriptLines {
					// skip errors on lines outside of the users
					// script file.
					continue
				}
				segs[1] = strconv.Itoa(n - scriptStartLine)
			}
			line = strings.Join(segs, ":")
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

type response struct {
	Value interface{}
	Error error
}

var scriptHarnessTemplate *template.Template
var scriptHarnessTemplateErr error
var scriptStartLine int

func init() {
	scriptHarnessTemplate, scriptHarnessTemplateErr = template.New("goscript").Parse(scriptHarnessCode)
	s := bufio.NewScanner(strings.NewReader(scriptHarnessCode))
	line := 1
	for s.Scan() {
		if s.Text() == "{{ .Goscript }}" {
			break
		}
		line++
	}
	scriptStartLine = line - 1
}

var scriptHarnessCode = `// Code generated by goscript; DO NOT EDIT
// github.com/matryer/goscript

package main

import (
	"encoding/gob"
	"os"
	"log"
)

// <goscript>
{{ .Goscript }}
// </goscript>

func main() {
	r := gob.NewDecoder(os.Stdin)
	w := gob.NewEncoder(os.Stdout)
	if err := w.Encode("ready"); err != nil {
		log.Fatalln(err)
	}
	for {
		var args []interface{}
		if err := r.Decode(&args); err != nil {
			log.Fatalln(err)
		}
		{{- range .InArgs }}
		{{- if .Variadic }}
		{{ .Name }} := make({{ .Typename }}, len(args)-{{ .Index }})
		for i := {{ .Index }}; i < len(args); i++ {
			{{ .Name }}[i-{{ .Index }}] = args[i].({{ .TypenameSingular }})
		}
		{{- else }}
		{{ .Name }} := args[{{ .Index }}].({{ .Typename }})
		{{- end }}
		{{- end }}
		var res response
		res.Value, res.Error = goscript({{ .ArgsList }})
		if err := w.Encode(res); err != nil {
			log.Fatalln(err)
		}
	}
}

type response struct {
	Value interface{}
	Error error
}
`
