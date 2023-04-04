package utils

import (
	"bufio"
	"fmt"
	"os"
)

// For logging only
const (
	DEFAULT_LOG_PATH = "/home/ubuntu/go/src/mycni/mycni.log"
)

// Only do logging
func Log(log ...string) {
	file, err := os.OpenFile(DEFAULT_LOG_PATH, os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		os.Create(DEFAULT_LOG_PATH)
	}

	defer file.Close()

	logRes := ""
	w := bufio.NewWriter(file)
	for _, c := range log {
		logRes += c
		logRes += " "
	}

	_, err = w.WriteString(logRes + "\r\n")
	if err != nil {
		fmt.Printf("Error occurred when writing the log!")
		return
	}

	w.Flush()
}
