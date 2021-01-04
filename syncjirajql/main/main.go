package main

import (
	"flag"
	"github.com/perolo/miro-scripts/syncjirajql"
)

func main() {
	propPtr := flag.String("prop", "gitlabmergestatus.properties", "a properties file")

	syncjirajql.SyncJiraJQL(*propPtr)
}
