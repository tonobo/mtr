package main

import (
	"fmt"

	"github.com/tonobo/mtr/cli"
)

func main() {
	err := cli.RootCmd.Execute()
	if err != nil {
		fmt.Println(err)
	}
}
