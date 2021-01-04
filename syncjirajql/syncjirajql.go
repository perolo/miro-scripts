package syncjirajql

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
	AppId           string `properties:"appid"`
	Bindusername    string `properties:"bindusername"`
	Bindpassword    string `properties:"bindpassword"`
	BoardName       string `properties:"boardname"`
}

var categoryLookup map[string]string

func SyncJiraJQL(propPtr string) {
//	propPtr := flag.String("prop", "confluence.properties", "a string")
	flag.Parse()
	p := properties.MustLoadFile(propPtr, properties.ISO_8859_1)
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
	categoryLookup = make(map[string]string)
	usernamelookup := make(map[string]string)
	useridlookup := make(map[string]string)
	for _, member := range members.Data {
		usernamelookup[member.User.ID] = member.User.Name
		useridlookup[member.User.Name] = member.User.ID
	}
	cardslookup := make(map[string]miro.WidgetResponseDataType)

	var boardid, boardname string
	for _, board := range boards.Data {
		boardid = board.ID
		boardname = board.Name
		fmt.Printf("Board Name: %s\n", board.Name)
		if boardname == cfg.BoardName {
			widgets, err := theClient.Widget.ListAllWidgets(context.Background(), board.ID)
			if err != nil {
				panic(err)
			}
			for _, widget := range widgets.Data {
				if widget.Type == "card" {
					widgetmeta, err := theClient.Widget.GetWidgetMetadata(context.Background(), board.ID, widget.ID)
					if err != nil {
						panic(err)
					}
					if widgetmeta.Issue != "" {
						if widgetmeta.YourAppID == cfg.AppId {
							fmt.Printf("  Widget Meta: %s\n", widgetmeta.Issue)
							cardslookup[widgetmeta.Issue] = widget
						} else {
							fmt.Printf("  Widget Meta from other AppID found: %s\n", widgetmeta.Issue)
						}
					}
				}
				//fmt.Printf("  Widget Type: %s\n", widget.Type)
				//fmt.Printf("  Widget Title: %s\n", widget.Title)
				//fmt.Printf("  Widget Description: %s\n", widget.Description)
				if _, ok := usernamelookup[widget.Assignee.UserID]; ok {
					fmt.Printf("  Widget Assignee: %s\n", usernamelookup[widget.Assignee.UserID])
				} else {
					fmt.Printf("  Unknown Assignee: %s\n", widget.Assignee.UserID)
				}
			}
		}
	}

	if boardname == cfg.BoardName {
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
			categoryLookup[issue.Fields.Status.Name] = issue.Fields.Status.StatusCategory.Name
			if _, ok := cardslookup[issue.Key]; ok {
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
				cardslookup[issue.Key] = miro.WidgetResponseDataType{
					ID:          resp.ID,
					Title:       title,
					Description: resp.Description,
					Assignee: struct {
						UserID string `json:"userId"`
					}{resp.Assignee.UserID},
					Style: struct {
					BackgroundColor string `json:"backgroundColor"`
					}{resp.Style.BackgroundColor},

				}
				var newMeta miro.WidgetMetadataType
				newMeta.Title = title
				newMeta.AppId = cfg.AppId
				newMeta.Issue = issue.Key
				newMeta.Status = issue.Fields.Status.Name
				lookupState(issue.Fields.Status.Name)
				respM, err := theClient.Widget.UpdateWidgetMetadata(context.Background(), boardid, resp.ID, &newMeta)
				if err != nil {
					panic(err)
				}
				fmt.Printf("  resp: %s\n", respM.Title)
/*
				cardslookup[issue.Key] = miro.WidgetResponseDataType{
					ID:          resp.ID,
					Title:       title,
					Description: resp.Description,

					Assignee: struct {
						UserID string `json:"userId"`
					}{resp.Assignee.UserID},
				}
*/
			}

		}
		for _, issue := range sres.Issues {
			title := getTitle(issue)
			if _, ok := cardslookup[issue.Key]; ok {
				wid := cardslookup[issue.Key]
				var issueAssignee string
				issueAssignee = ""
				if issue.Fields.Assignee != nil {
					issueAssignee = issue.Fields.Assignee.DisplayName
				}
				if issueAssignee == usernamelookup[wid.Assignee.UserID] {
					fmt.Printf("Already right assignee: %s\n", title)
				} else {
					var changeAssignee miro.SimpleCardAssignee
					//var resp *miro.CreateCardRespType
					if _, ok := useridlookup[issue.Fields.Assignee.DisplayName]; ok {
						changeAssignee.Assignee.UserID = useridlookup[issueAssignee]
					} else {

						changeAssignee.Assignee.UserID = ""
					}
					if wid.Assignee.UserID == "" && changeAssignee.Assignee.UserID == "" {
						fmt.Printf("Do nothing - Assignee unknown in Miro: %s\n", issue.Fields.Assignee.DisplayName)
					} else {
						_, err = theClient.Widget.UpdateAssigneeCard(context.Background(), boardid, wid.ID, &changeAssignee)
						if err != nil {
							panic(err)
						}
						//fmt.Printf("  resp: %s\n", resp.Title)
					}
				}

				if wid.Style.BackgroundColor == lookupState(issue.Fields.Status.Name) {
					fmt.Printf("Already right color: %s\n", title)
				} else {
					var changeStyle miro.SimpleCardStyle
					changeStyle.Style.BackgroundColor = lookupState(issue.Fields.Status.Name)
					_, err = theClient.Widget.UpdateStyleCard(context.Background(), boardid, wid.ID, &changeStyle)
					if err != nil {
						panic(err)
					}

				}

			} else {
				panic(err)
			}
		}
	} else {
		fmt.Printf("Board Not Found: %s\n", boardname)
	}
}
func lookupState(status string) string{
	var color string
	//TO DO kinda backwards, but I wanted to keep the options to add a Tag=status when this is added...
	if categoryLookup[status] == "Done" {
		color = "#008000"
	} else if categoryLookup[status] == "In Progress" {
		color = "#FFFF00"
	} else 	if categoryLookup[status] == "To Do" {
		color = "#0000ff"
	} else {
		panic(fmt.Errorf("illegal Status %s", status))
	}
	return color
}

func getTitle(issue jira.Issue) string {
	u, err := url.Parse(issue.Self)
	if err != nil {
		panic(err)
	}
	title := "<p><a href=\"https://" + u.Host + "/browse/" + issue.Key + "\">[" + issue.Key + "] " + issue.Fields.Summary + "</a></p>"
	return title
}
