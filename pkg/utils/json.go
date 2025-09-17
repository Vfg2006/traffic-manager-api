package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
)

func PrettyJson(in any) string {
	var buffer []byte
	var err error

	if reflect.TypeOf(in) != reflect.TypeOf([]byte{}) {
		buffer, err = json.Marshal(in)
		if err != nil {
			fmt.Println(err)
		}
	} else {
		buffer = in.([]byte)
	}

	var out bytes.Buffer
	err = json.Indent(&out, buffer, "", "\t")
	if err != nil {
		fmt.Println(err)
	}

	return out.String()
}
