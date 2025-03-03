package main

import (
	"fmt"
	"log"
	"reflect"

	"github.com/sandertv/gophertunnel/minecraft"
)

// CheckDialerFields outputs the available fields in minecraft.Dialer
// This is useful for troubleshooting when the gophertunnel API changes
func CheckDialerFields() {
	dialer := minecraft.Dialer{}
	dialerType := reflect.TypeOf(dialer)
	log.Println("Available fields in minecraft.Dialer:")
	
	for i := 0; i < dialerType.NumField(); i++ {
		field := dialerType.Field(i)
		log.Printf("  %s: %s", field.Name, field.Type.String())
	}
}

// TryClientDataTypes attempts to identify the correct ClientData type
func TryClientDataTypes() {
	// Try different types that might be used for client data
	dialer := minecraft.Dialer{}
	
	// Check if ClientData field exists and what type it expects
	t := reflect.TypeOf(dialer)
	field, found := t.FieldByName("ClientData")
	
	if found {
		log.Printf("Found ClientData field of type: %s", field.Type.String())
		
		// Output the structure of the ClientData type
		clientDataType := field.Type
		log.Println("Fields in ClientData type:")
		
		// If it's a struct, print its fields
		if clientDataType.Kind() == reflect.Struct {
			for i := 0; i < clientDataType.NumField(); i++ {
				f := clientDataType.Field(i)
				log.Printf("  %s: %s", f.Name, f.Type.String())
			}
		}
	} else {
		log.Println("ClientData field not found in minecraft.Dialer")
	}
}

func init() {
	// This will run when the program starts
	fmt.Println("======= GOPHERTUNNEL VERSION CHECK =======")
	CheckDialerFields()
	TryClientDataTypes()
	fmt.Println("=========================================")
}
