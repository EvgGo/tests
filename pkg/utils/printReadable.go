package utils

import (
	"encoding/json"
	"fmt"
)

func PrintReadable(data interface{}) {
	prettyData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		fmt.Println("Error formatting data:", err)
		return
	}
	fmt.Println(string(prettyData))
}
