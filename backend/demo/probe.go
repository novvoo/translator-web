package main

import (
	"fmt"
	"reflect"
	"github.com/phpdave11/gofpdi"
)

func main() {
	imp := gofpdi.NewImporter()
	t := reflect.TypeOf(imp)
	for i := 0; i < t.NumMethod(); i++ {
		m := t.Method(i)
		fmt.Printf("Method: %s %v\n", m.Name, m.Type)
	}
}
