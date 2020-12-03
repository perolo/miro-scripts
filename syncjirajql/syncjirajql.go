package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/magiconair/properties"
	"github.com/perolo/go-miro/miro"
	"github.com/perolo/jira-client"
	"log"
	"net/url"
	"strings"
	"time"
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
	usernamelookup := make(map[string]string)
	useridlookup := make(map[string]string)
	for _, member := range members.Data {
		usernamelookup[member.User.ID] = member.User.Name
		useridlookup[member.User.Name] = member.User.ID
	}
	cardslookup := make(map[string]miro.WidgetResponseDataType)

	var boardid string
	for _, board := range boards.Data {
		boardid = board.ID
		fmt.Printf("Board Name: %s\n", board.Name)

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
			if _, ok := usernamelookup[widget.Assignee.UserID]; ok {
				fmt.Printf("  Widget Assignee: %s\n", usernamelookup[widget.Assignee.UserID])
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
		title := getTitle(issue)
		if _, ok := cardslookup[title]; ok {
			fmt.Printf("Already on board: %s\n", title)
		} else {
			newCard2 := miro.SimpleCard{
				Type:        "card",
				Title:       title,
				Description: issue.Fields.Description,
			}
			resp, err := theClient.Widget.CreateSimpleCard(context.Background(), boardid, &newCard2)
			if err != nil {
				panic(err)
			}
			fmt.Printf("  resp: %s\n", resp.Title)
			cardslookup[title] = miro.WidgetResponseDataType{
				ID:          resp.ID,
				Title:       resp.Title,
				Description: resp.Description,
				Assignee: struct {
					UserID string `json:"userId"`
				}{resp.Assignee.UserID},
			}
		}

	}
	for _, issue := range sres.Issues {
		title := getTitle(issue)
		if _, ok := cardslookup[title]; ok {
			wid := cardslookup[title]
			var issueAssignee string
			issueAssignee = ""
			if issue.Fields.Assignee != nil {
				issueAssignee = issue.Fields.Assignee.DisplayName
			}
			if issueAssignee == usernamelookup[wid.Assignee.UserID] {
				fmt.Printf("Already right assignee: %s\n", title)
			} else {
				var changeAssignee miro.SimpleCardAssignee
				var resp *miro.CreateCardRespType
				if _, ok := useridlookup[issue.Fields.Assignee.DisplayName]; ok {
					changeAssignee.Assignee.UserID = useridlookup[issueAssignee]
				} else {

					changeAssignee.Assignee.UserID = ""
				}
				if wid.Assignee.UserID == "" && changeAssignee.Assignee.UserID== "" {
					fmt.Printf("Do nothing - Assignee unknown in Miro: %s\n", issue.Fields.Assignee.DisplayName)

				} else {
					resp, err = theClient.Widget.UpdateAssigneeCard(context.Background(), boardid, wid.ID, &changeAssignee)
					if err != nil {
						panic(err)
					}
					fmt.Printf("  resp: %s\n", resp.Title)
				}
			}
		} else {
			panic(err)
		}
	}
}

func getTitle(issue jira.Issue) string {
	u, err := url.Parse(issue.Self)
	if err != nil {
		panic(err)
	}
	//browse/STP-346
	title := "<p><a href=\"https://" + u.Host + "/browse/" + issue.Key + "\">[" + issue.Key + "] " + issue.Fields.Summary + "</a></p>"
	//<p><a href="https://www.dn.se/">SimpleCard Title</a></p>
	return title
}
