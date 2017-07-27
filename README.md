# goscript

Goscript: Runtime execution of Go code.

## Usage

The script string must define a `goscript` function that takes in zero or more
arguments, and returns two; a value and an error.

You then start the script like this:

```go
script := goscript.New(`
	
	import (
		"strings"
	)
	
	func goscript(name string) (string, error) {
		return "Hello " + strings.ToUpper(name), nil
	}
	
`)
defer script.Close()
```

And make calls to the `goscript` function like this:

```go
greeting, err := script.Execute("Mat")
if err != nil {
	log.Fatalln(err)
}
log.Println(greeting)

// prints: Hello MAT
```

## Rules

* Every script must provide a `goscript` entry function
* Imports must be included above the `goscript` function if required
* Any special types being used as input or output require `gob.Register` in the script and the calling code
* The `goscript` function must return two values and the second type must be `error`
* Only execute trusted code; there are no limits to what scripts can do

## How it works

* Goscript generates a mini Go program and executes it with `go run`
* The script program communicates with the host program via stdin/stdout
* Values are encoded/decoded via the `encoding/gob` package
* The script program stays running until `Close` is called
