package iofs

import (
	"fmt"
	"log"
)

func ExampleReadFileFS() {
	type MyCustomType struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	// Read and print a file's content.
	file, err := ReadFileFS[MyCustomType](testFiles, "file1.json")
	if err != nil {
		log.Fatal(err)
	}
	// Print the file's content as string
	fmt.Println(file.String())

	// Parses the JSON-encoded data and returns the result
	myFile := file.JsonOrDie()
	fmt.Println(myFile.Name, myFile.Type)

	// Print the file's content as []byte
	fmt.Println(file.Bytes())

	// Output:
	// {"name": "foo", "type": "json"}
	// foo json
	// [123 34 110 97 109 101 34 58 32 34 102 111 111 34 44 32 34 116 121 112 101 34 58 32 34 106 115 111 110 34 125]
}
