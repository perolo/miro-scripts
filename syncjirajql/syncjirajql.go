package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/magiconair/properties"
	"github.com/perolo/go-miro/miro"
	"log"
	"strings"
	"time"
	"github.com/perolo/jira-client"
)

// or through Decode
type Config struct {
	Host            string `properties:"host"`
	User            string `properties:"user"`
	Pass            string `properties:"password"`
	Token           string `properties:"token"`
	Simple          bool   `properties:"simple"`
	AddOperation    bool   `properties:"add"`
	RemoveOperation bool   `properties:"remove"`
	Report          bool   `properties:"report"`
	Limited         bool   `properties:"limited"`
	AdGroup         string `properties:"adgroup"`
	Localgroup      string `properties:"localgroup"`
	File            string `properties:"file"`
	JQL             string `properties:"jql"`
	Bindusername    string `properties:"bindusername"`
	Bindpassword    string `properties:"bindpassword"`
}

func main() {
	propPtr := flag.String("prop", "confluence.properties", "a string")
	flag.Parse()
	p := properties.MustLoadFile(*propPtr, properties.ISO_8859_1)
	var cfg Config
	if err := p.Decode(&cfg); err != nil {
		log.Fatal(err)
	}
	theClient := miro.NewClient(cfg.Token)
	_, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	auth, err := theClient.AuthzInfo.Get(context.Background())
	if err != nil {
		panic(err)
	}

	boards, err := theClient.Boards.GetCurrentUserBoards(context.Background(), auth.Team.ID)
	if err != nil {
		panic(err)
	}
	members, err := theClient.Teams.ListTeamMembers(context.Background(), auth.Team.ID)
	if err != nil {
		panic(err)
	}
	userslookup := make(map[string]string)
	for _, member := range members.Data {
		userslookup[member.User.ID] = member.User.Name
	}
	cardslookup := make(map[string]miro.WidgetResponseDataType)

	var boardid string
	for _, board := range boards.Data {
		boardid = board.ID
		fmt.Printf("Board Name: %s\n", board.Name)
		//https://api.miro.com/v1/boards/id/widgets/
		widgets, err := theClient.Widget.ListAllWidgets(context.Background(), board.ID)
		if err != nil {
			panic(err)
		}
		for _, widget := range widgets.Data {
			if widget.Type == "card" {
				cardslookup[widget.Title] = widget
			}
			fmt.Printf("  Widget Type: %s\n", widget.Type)
			fmt.Printf("  Widget Title: %s\n", widget.Title)
			fmt.Printf("  Widget Description: %s\n", widget.Description)
			if _, ok := userslookup[widget.Assignee.UserID]; ok {
				fmt.Printf("  Widget Assignee: %s\n", userslookup[widget.Assignee.UserID])
			} else {
				fmt.Printf("  Unknown Assignee: %s\n", widget.Assignee.UserID)
			}
		}
	}

	tp := jira.BasicAuthTransport{
		Username: strings.TrimSpace(cfg.User),
		Password: strings.TrimSpace(cfg.Pass),
	}
	jiraClient, err := jira.NewClient(tp.Client(), strings.TrimSpace(cfg.Host))
	if err != nil {
		fmt.Printf("\nerror: %v\n", err)
		panic(err)
	}
	sres, _, err := jiraClient.Issue.Search(cfg.JQL, &jira.SearchOptions{StartAt: 0, MaxResults: 10})
	if err != nil {
		panic(err)
	}
	for _, issue := range sres.Issues {
		title := "[" + issue.Key + "] " + issue.Fields.Summary
		if _, ok := cardslookup[title]; ok {
			fmt.Printf("Already on board: %s\n", title)
		} else {
			newCard2 := miro.Card{
				Type:        "card",
				Title:       title,
				Description: issue.Fields.Description,
			}
			resp, err := theClient.Widget.CreateCard(context.Background(), boardid, &newCard2)
			if err != nil {
				panic(err)
			}
			fmt.Printf("  resp: %s\n", resp)
		}
	}

}
