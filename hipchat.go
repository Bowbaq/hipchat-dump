package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"sort"
	"strconv"
	"time"

	"github.com/codegangsta/cli"
	"github.com/mitchellh/go-homedir"
	"github.com/tbruyelle/hipchat-go/hipchat"
)

var (
	Version     string
	apiPageSize = 1000
)

type archive struct {
	Users         []hipchat.User
	Conversations map[string][]*hipchat.Message
}

func main() {
	app := cli.NewApp()
	app.Name = "hipchat"
	app.Usage = "Archive your HipChat private messages and search them"
	app.HideVersion = true

	app.Commands = []cli.Command{
		{
			Name:    "dump",
			Aliases: []string{"d"},
			Usage:   "Archive your HipChat private messages",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "token, t",
					Usage: "(required) HipChat auth token with view_group, view_messages scope.\n\tSee https://www.hipchat.com/account/api",
				},
				cli.StringFlag{
					Name:  "filename, f",
					Usage: "Path of the file where the archive will be written",
				},
			},
			Action: func(c *cli.Context) {
				if !c.IsSet("token") {
					cli.ShowSubcommandHelp(c)
					return
				}

				filename := c.String("filename")
				if filename == "" {
					filename = defaultArchivePath()
					check(os.MkdirAll(path.Dir(filename), 0755))
				}

				check(dumpMessages(c.String("token"), filename))
				fmt.Println("Archive was written at", filename)
			},
		},
	}

	app.Run(os.Args)
}

func dumpMessages(token, filename string) error {
	h := hipchat.NewClient(token)

	users, err := getUsers(h)
	if err != nil {
		return err
	}

	conversations := make(map[string][]*hipchat.Message)
	for _, user := range users {
		conversations[strconv.Itoa(user.ID)] = getMessages(h, user.ID)
	}

	encoded, err := json.MarshalIndent(archive{
		Users:         users,
		Conversations: conversations,
	}, "", "    ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filename, encoded, 0655)
}

func getUsers(h *hipchat.Client) ([]hipchat.User, error) {
	fmt.Print("Getting users")
	opt := &hipchat.UserListOptions{
		ListOptions: hipchat.ListOptions{
			MaxResults: apiPageSize,
		},
	}
	users, res, err := h.User.List(opt)
	for res.StatusCode == 429 { // Retry while rate-limited
		fmt.Printf(" - rate-limited, sleeping for 15s\nGetting users")
		time.Sleep(15 * time.Second)
		users, res, err = h.User.List(opt)
	}
	fmt.Printf(" - Done [%d]\n", len(users))

	return users, err
}

type byMostRecent []*hipchat.Message

func (msgs byMostRecent) Len() int           { return len(msgs) }
func (msgs byMostRecent) Less(i, j int) bool { return msgs[i].Date > msgs[j].Date }
func (msgs byMostRecent) Swap(i, j int)      { msgs[i], msgs[j] = msgs[j], msgs[i] }

func getMessages(h *hipchat.Client, userID int) []*hipchat.Message {
	fmt.Printf("Getting messages for %d", userID)

	uniqueMessages := getMessagesPage(h, userID, "recent", 0)
	if len(uniqueMessages) == 0 {
		fmt.Println(" - Done [0]")
		return []*hipchat.Message{}
	}

	now := time.Now().Add(-1 * time.Minute).UTC().Format(time.RFC3339)
	start := 0
	for {
		page := getMessagesPage(h, userID, now, start)
		for _, msg := range page {
			uniqueMessages[msg.ID] = msg
		}

		if len(page) < apiPageSize {
			break
		}

		start += len(page) - 1
	}

	var messages []*hipchat.Message
	for _, msg := range uniqueMessages {
		messages = append(messages, msg)
	}
	sort.Sort(byMostRecent(messages))
	fmt.Printf(" - Done [%d]\n", len(messages))

	return messages
}

func getMessagesPage(h *hipchat.Client, userID int, date string, startIndex int) map[string]*hipchat.Message {
	u := fmt.Sprintf("user/%d/history", userID)
	opt := &hipchat.HistoryOptions{
		ListOptions: hipchat.ListOptions{
			MaxResults: apiPageSize,
			StartIndex: startIndex,
		},
		Date:    date,
		Reverse: false,
	}

	req, err := h.NewRequest("GET", u, opt, nil)
	if err != nil {
		log.Println(req.URL.String(), err)
		return nil
	}

	var result hipchat.History
	res, err := h.Do(req, &result)
	for res.StatusCode == 429 { // Retry while rate-limited
		fmt.Printf(" - rate-limited, sleeping for 15s\nGetting messages for %d", userID)
		time.Sleep(15 * time.Second)
		res, err = h.Do(req, &result)
	}
	if err != nil {
		log.Println(req.URL.String(), err)
		return nil
	}

	messages := make(map[string]*hipchat.Message)
	for i, msg := range result.Items {
		messages[msg.ID] = &result.Items[i]
	}

	return messages
}

func defaultArchivePath() string {
	home, err := homedir.Dir()
	check(err)
	return path.Join(home, ".hipchat", "archive.json")
}

func check(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}
