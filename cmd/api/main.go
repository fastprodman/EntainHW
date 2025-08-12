package main

import "os"

func main() {
	_, err := readConfig()
	if err != nil {
		os.Exit(1)
	}
}
