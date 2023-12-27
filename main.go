package main

import (
	"fmt"

	"dcrcli/mongo"
	"dcrcli/mongosh"
)

func main() {
	// prefer mongosh first if available else fallback to mongo shell and getMongoData
	if !mongosh.Detect() {
		if !mongo.Detect() {
			fmt.Println(
				"O Oh: Could not find the mongosh or legacy mongo shell. Install that first.",
			)
		}
		mongo.Run()
	}
	mongosh.Run()
}
