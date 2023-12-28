package main

import (
	"dcrcli/mongosh"
)

func main() {
	// prefer mongosh first if available else fallback to mongo shell and getMongoData
	/**	if mongosh.Detect() {
		mongosh.Runshell()
	} else if mongo.Detect() {
		fmt.Println("mongosh not present locating legacy mongo shell.")
		mongo.Runshell()
	} else {
		fmt.Println(
			"O Oh: Could not find the mongosh or legacy mongo shell. Install that first.",
		)
	}
	*/
	mongosh.Runshell()
}
