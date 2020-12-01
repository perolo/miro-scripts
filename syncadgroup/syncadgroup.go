package main

import (
	"flag"
	"fmt"
	"github.com/magiconair/properties"
	"github.com/perolo/ad-utils"
	excelutils "github.com/perolo/excel-utils"
    "github.com/perolo/go-miro/miro"
	"log"
	"time"
	"context"
)

// or through Decode
type Config struct {
	User            string `properties:"user"`
	Token        string `properties:"token"`
	Simple       bool   `properties:"simple"`
	AddOperation bool   `properties:"add"`
	RemoveOperation bool   `properties:"remove"`
	Report       bool   `properties:"report"`
	Limited      bool   `properties:"limited"`
	AdGroup      string `properties:"adgroup"`
	Localgroup   string `properties:"localgroup"`
	File         string `properties:"file"`
	Bindusername string `properties:"bindusername"`
	Bindpassword string `properties:"bindpassword"`
}

func initReport(cfg Config) {
	if cfg.Report {
		excelutils.NewFile()
		excelutils.SetCellFontHeader()
		excelutils.WiteCellln("Introduction")
		excelutils.WiteCellln("Please Do not edit this page!")
		excelutils.WiteCellln("This page is created by the projectreport script: github.com\\perolo\\miro-scripts\\SyncADGroup")
		t := time.Now()
		excelutils.WiteCellln("Created by: " + cfg.User + " : " + t.Format(time.RFC3339))
		excelutils.WiteCellln("")
		excelutils.WiteCellln("The Report Function shows:")
		excelutils.WiteCellln("   AdNames - Name and user found in AD Group")
		excelutils.WiteCellln("   JIRA Users - Name and user found in JIRA Group")
		excelutils.WiteCellln("   Not in AD - Users in the Local Group not found in the AD")
		excelutils.WiteCellln("   Not in JIRA - Users in the AD not found in the JIRA Group")
		excelutils.WiteCellln("   AD Errors - Internal error when searching for user in AD")
		excelutils.WiteCellln("")

		excelutils.SetCellFontHeader2()
		excelutils.WiteCellln("Group Mapping")
		if cfg.Simple {
			excelutils.WriteColumnsHeaderln([]string{"AD Group", "Local group"})
			excelutils.WriteColumnsln([]string{cfg.AdGroup, cfg.Localgroup})
		} else {
			excelutils.WriteColumnsHeaderln([]string{"AD Group", "Local group"})
			for _, syn := range GroupSyncs {
				excelutils.WriteColumnsln([]string{syn.AdGroup, syn.LocalGroup})
			}
		}
		excelutils.WiteCellln("")
		excelutils.SetCellFontHeader2()
		excelutils.WiteCellln("Report")

		excelutils.AutoFilterStart()
		var headers = []string{"Report Function", "AD group", "Local Group", "Name", "Uname", "Mail","Error"}
		excelutils.WriteColumnsHeaderln(headers)
	}
}

func endReport(cfg Config) {
	if cfg.Report {
		file := fmt.Sprintf(cfg.File, "-Miro")
		excelutils.AutoFilterEnd()
		excelutils.SaveAs(file)
	}
}
func main() {
	propPtr := flag.String("prop", "confluence.properties", "a string")
	flag.Parse()
	p := properties.MustLoadFile(*propPtr, properties.ISO_8859_1)
	var cfg Config
	if err := p.Decode(&cfg); err != nil {
		log.Fatal(err)
	}
	toolClient := toollogin(cfg)
	initReport(cfg)
	ad_utils.InitAD(cfg.Bindusername, cfg.Bindpassword)
	if cfg.Simple {
		SyncGroupInTool(cfg, toolClient)
	} else {
		for _, syn := range GroupSyncs {
			cfg.AdGroup = syn.AdGroup
			cfg.Localgroup = syn.LocalGroup
			SyncGroupInTool(cfg, toolClient)
		}
	}
	endReport(cfg)
	ad_utils.CloseAD()
}

func toollogin(cfg Config) *miro.Client {

	c := miro.NewClient(cfg.Token)
	_, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()


	return c
}

func SyncGroupInTool(cfg Config, client *miro.Client) {
	var toolGroupMemberNames map[string]ad_utils.ADUser
	fmt.Printf("\n")
	fmt.Printf("SyncGroup AdGroup: %s LocalGroup: %s \n", cfg.AdGroup, cfg.Localgroup)
	fmt.Printf("\n")
	var adUnames, aderrs []ad_utils.ADUser
	if cfg.AdGroup != "" {
		adUnames, _, aderrs = ad_utils.GetUnamesInGroup(cfg.AdGroup)
		fmt.Printf("adUnames(%v): %s \n", len(adUnames), adUnames)
	}
	if cfg.Report {
		if !cfg.Limited {
			for _, adu := range adUnames {
				var row = []string{"AD Names", cfg.AdGroup, cfg.Localgroup, adu.Name, adu.Uname, adu.Mail, adu.Err}
				excelutils.WriteColumnsln(row)
			}
		}
		for _, aderr := range aderrs {
			var row = []string{"AD Errors", cfg.AdGroup, cfg.Localgroup, aderr.Name, aderr.Uname, aderr.Mail, aderr.Err}
			excelutils.WriteColumnsln(row)
		}
	}
	if cfg.Localgroup != "" {
		toolGroupMemberNames = getUnamesInToolGroup(client, cfg.Localgroup)
		if cfg.Report {
			if !cfg.Limited {
				for _, tgm := range toolGroupMemberNames {
					var row = []string{"Miro Users", cfg.AdGroup, cfg.Localgroup, tgm.Name, tgm.Uname, tgm.Mail, tgm.Err}
					excelutils.WriteColumnsln(row)
				}
			}
		}
	}
	if cfg.Localgroup != "" && cfg.AdGroup != "" {
		notInTool := ad_utils.Difference(adUnames, toolGroupMemberNames)
		fmt.Printf("not In Miro(%v): %s \n", len(notInTool), notInTool)
		if cfg.Report {
			for _, nji := range notInTool {
				var row = []string{"AD group users not found in Tool user group", cfg.AdGroup, cfg.Localgroup, nji.Name, nji.Uname, nji.Mail, nji.Err}
				excelutils.WriteColumnsln(row)
			}
		}
		notInAD := ad_utils.Difference2(toolGroupMemberNames, adUnames)
		fmt.Printf("notInAD: %s \n", notInAD)
		if cfg.Report {
			for _, nad := range notInAD {
				var row = []string{"Tool user group member not found in AD", cfg.AdGroup, cfg.Localgroup, nad.Name, nad.Uname, nad.Mail, nad.Err}
				excelutils.WriteColumnsln(row)
			}
		}
		if cfg.AddOperation {
			for _, notin := range notInTool {
				fmt.Printf( "Add user Not Implemented. Group: %s status: %s \n", cfg.Localgroup, notin)
/*
				_, _, err := client.Group.Add(cfg.Localgroup, notin.Uname)
				if err != nil {
					fmt.Printf("Failed to add user. Group: %s status: %s \n", cfg.Localgroup, notin)
				}

 */
			}
		}
		if cfg.RemoveOperation {
			for _, notin := range notInAD {
				fmt.Printf("Remove user Not Implemented. Group: %s status: %s \n", cfg.Localgroup, notin)
/*				 _, err := client.Group.Remove(cfg.Localgroup, notin.Uname)
				if err != nil {
					fmt.Printf("Failed to remove user. Group: %s status: %s \n", cfg.Localgroup, notin)
				}

 */
			}
		}
	}
}

func getUnamesInToolGroup(theClient *miro.Client, localgroup string) map[string]ad_utils.ADUser {
	groupMemberNames := make(map[string]ad_utils.ADUser)
	auth, err := theClient.AuthzInfo.Get(context.Background())
	if err != nil {
		panic(err)
	}

	members, err := theClient.Teams.ListTeamMembers(context.Background(), auth.Team.ID)
	if err != nil {
		panic(err)
	}

	for _, member := range members.Data {
		var newUser ad_utils.ADUser
		newUser.Name = member.User.Name
		groupMemberNames[member.User.Name] = newUser
	}

	return groupMemberNames
}
