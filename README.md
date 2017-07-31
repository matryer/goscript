![Goscript logo by Rob Baines](goscript-logo-small.png)

# goscript

Goscript: Runtime execution of Go code.

## Usage

A Goscript is a string that contains valid Go code that can be executed by Goscript.

The script string defines a `goscript` function that takes in zero or more
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

## Security

Running any Go code unsupervised represents a pretty giant security concern for obvious reasons, but that doesn't
nncessarily spell the end for using Goscript in your projects.

One option is to wrap Goscript and provide the `goscript` func signature youself, and allow users to only provide the
body. This would also prevent them from controlling the imports too. And with some simple string checking, you'd be able
to protect from injection attacks.

For an example of how this might work, see the `example/rename` tool.

## How it works

* Goscript generates a mini Go program and executes it with `go run`
* The script program communicates with the host program via stdin/stdout
* Values are encoded/decoded via the `encoding/gob` package
* The script program stays running until `Close` is called

---

Logo by [Rob Baines](https://twitter.com/telecoda), inspired by Renee French, licensed under a [Creative Commons Attribution 4.0 International license](https://creativecommons.org/licenses/by/4.0/).