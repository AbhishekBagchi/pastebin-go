[![Build Status](https://travis-ci.com/AbhishekBagchi/pastebin-go.svg?token=GGuKjubCYgHZZRYomaqG&branch=master)](https://travis-ci.com/AbhishekBagchi/pastebin-go)

Pastebin server in Go.

A representative set of steps to get this up and running would be as follows

 - `go get -v`  [`github.com/AbhishekBagchi/pastebin-go`](https://github.com/AbhishekBagchi/pastebin-go)
 - `cd ${GOPATH}/src/github.com/AbhishekBagchi/pastebin-go`
 - `go build`
 - `./pastebin-go --static-dir ./static --template-dir ./tmpl --interface localhost:8000`
 
 The only other available command line option is `--clean-database`, and all that does is delete any pre-existing database file. 
