package main

import (
	"github.com/avitaltamir/cyphernetes/pkg/parser"
)

var resourceSpecs = make(map[string][]string)

func initResourceSpecs() {
	specs, err := parser.GetOpenAPIResourceSpecs()
	if err != nil {
		// fmt.Println("Error fetching resource specs:", err)
		return
	}
	resourceSpecs = specs
}
