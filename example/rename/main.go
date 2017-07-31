package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/matryer/goscript"
)

const usage = `usage: rename <script> <files...>

  script  Goscript to assign 'out' variable to mutation of 'filename'
  	      e.g. 'out=strings.ToUpper(filename)'
  files   List of files to rename (e.g. '*.go')
`

func main() {
	if len(os.Args) < 3 {
		fmt.Println(usage)
		os.Exit(1)
	}
	script := os.Args[1]
	files := os.Args[2:]
	if err := checkDisallowed(script); err != nil {
		log.Fatalln(err)
	}
	fullscript := `
import (` + imports(script) + `)

func goscript(filename string) (string, error) {
		var out string
		` + script + `
		return out, nil
}
	`
	gs := goscript.New(fullscript)
	defer gs.Close()
	for _, file := range files {
		newfilename, err := gs.Execute(file)
		if err != nil {
			log.Fatalln("goscript:", err)
		}
		log.Println(file, "->", newfilename)
		// NOTE: This doesn't actually do the rename, it's just an example program
	}
}

var disallowedStrings = []string{"import", "func"}

func checkDisallowed(script string) error {
	loscript := strings.ToLower(script)
	for _, s := range disallowedStrings {
		if strings.Contains(loscript, strings.ToLower(s)) {
			return fmt.Errorf("not allowed: %q", s)
		}
	}
	return nil
}

var allowedImports = []string{"filepath", "strings"}

// imports generates a list of allowedImports if they are used in
// the script.
func imports(script string) string {
	var imports string
	for _, pkg := range allowedImports {
		if strings.Contains(script, fmt.Sprintf("%s.", pkg)) {
			imports += fmt.Sprintf("%q\n", pkg)
		}
	}
	return imports
}
