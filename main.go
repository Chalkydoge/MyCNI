package main

import (
	"fmt"
	"mycni/utils"
)

// func testClient() {

// }

func main() {
	res, err := utils.GetMasterNodeIP()
	if err != nil {
		fmt.Printf("Error occurred while reading master node ip %v", err.Error())
		return
	}
	fmt.Println(res)
}