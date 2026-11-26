// Command tribrach-vet runs the tribrach analyzer under go vet:
//
//	go vet -vettool=$(which tribrach-vet) ./...
//
// It can also run on its own: tribrach-vet ./...
package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"

	"github.com/tribrach-dev/tribrach-go/analyzer"
)

func main() { singlechecker.Main(analyzer.Analyzer) }
