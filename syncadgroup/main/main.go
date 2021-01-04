package main

import (
	"flag"
	"github.com/perolo/miro-scripts/syncadgroup"
)

func main() {
	propPtr := flag.String("prop", "gitlabmergestatus.properties", "a properties file")

	syncadgroup.MiroAdGroup(*propPtr)
}
