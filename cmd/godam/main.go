package main

import (
	"log"
	"os"
)

func main() {
	initDependencies()
	if err := rootCmd.Execute(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
