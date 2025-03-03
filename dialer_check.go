package main

import (
	"fmt"
	"log"
	"reflect"
	"strings"

	"github.com/sandertv/gophertunnel/minecraft"
)

func PrintStructFields(structValue interface{}, depth int) {
	indent := strings.Repeat("  ", depth)
	t := reflect.TypeOf(structValue)
	v := reflect.ValueOf(structValue)
	if t.Kind() != reflect.Struct {
		log.Printf("%sNot a struct: %v", indent, t.Kind())
		return
	}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		fieldValue := v.Field(i)
		log.Printf("%s%s: %s", indent, field.Name, field.Type.String())
		if field.Type.Kind() == reflect.Struct && depth < 3 {
			log.Printf("%s  {", indent)
			PrintStructFields(fieldValue.Interface(), depth+1)
			log.Printf("%s  }", indent)
		}
	}
}

func CheckDialerStructure() {
	fmt.Println("\n==== DIALER STRUCTURE DIAGNOSTICS ====")
	dialer := minecraft.Dialer{}
	log.Println("Dialer struct fields:")
	PrintStructFields(dialer, 1)
	fmt.Println("==== END DIAGNOSTICS ====\n")
}

func RunDialerCheck() {
	CheckDialerStructure()
}
