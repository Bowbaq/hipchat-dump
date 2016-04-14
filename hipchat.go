package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
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
	Users         map[string]*hipchat.User
	Conversations map[string][]*hipchat.Message
}

func main() {
	app := cli.NewApp()
	app.Name = "hipchat"
	app.Usage = "Archive your HipChat private messages and search them"
	app.Version = Version
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
					Usage: "Path where the archive will be written. Defaults to " + defaultArchivePath(),
				},
				cli.BoolFlag{
					Name:  "include-deleted-users, d",
					Usage: "Set this flag to include conversations with deleted users. You may need additional permissions.",
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
				}

				check(dumpMessages(c.String("token"), filename, c.Bool("include-deleted-users")))
				fmt.Println("Archive was written at", filename)
			},
		},
	}
	app.Run(os.Args)
}

func dumpMessages(token, filename string, includeDeleted bool) error {
	h := hipchat.NewClient(token)

	fmt.Println("Fetching data from the HipChat API. This may take several minutes")

	users, err := getUsers(h, includeDeleted)
	if err != nil {
		return err
	}

	conversations := make(map[string][]*hipchat.Message)
	for _, user := range users {
		conversations[strconv.Itoa(user.ID)] = getMessages(h, user)
	}

	return writeArchive(users, conversations, filename)
}

func getUsers(h *hipchat.Client, includeDeleted bool) (map[string]*hipchat.User, error) {
	fmt.Print("Getting users")
	opt := &hipchat.UserListOptions{
		ListOptions: hipchat.ListOptions{
			MaxResults: apiPageSize,
		},
		IncludeDeleted: includeDeleted,
	}
	users, res, err := h.User.List(opt)
	for res.StatusCode == 429 { // Retry while rate-limited
		// fmt.Printf(" - rate-limited, sleeping for 15s\nGetting users")
		time.Sleep(15 * time.Second)
		users, res, err = h.User.List(opt)
	}
	fmt.Printf(" - Done [%d]\n", len(users))

	usersByID := make(map[string]*hipchat.User)
	for i, user := range users {
		usersByID[strconv.Itoa(user.ID)] = &users[i]
	}

	return usersByID, err
}

type byLeastRecent []*hipchat.Message

func (msgs byLeastRecent) Len() int           { return len(msgs) }
func (msgs byLeastRecent) Less(i, j int) bool { return msgs[i].Date < msgs[j].Date }
func (msgs byLeastRecent) Swap(i, j int)      { msgs[i], msgs[j] = msgs[j], msgs[i] }

func getMessages(h *hipchat.Client, user *hipchat.User) []*hipchat.Message {
	fmt.Printf("Getting conversation with %s", username(user))

	uniqueMessages := getMessagesPage(h, user, "recent", 0)
	if len(uniqueMessages) == 0 {
		fmt.Println(" - Done [0 messages]")
		return []*hipchat.Message{}
	}

	now := time.Now().Add(-1 * time.Minute).UTC().Format(time.RFC3339)
	start := 0
	for {
		page := getMessagesPage(h, user, now, start)
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
	sort.Sort(byLeastRecent(messages))
	fmt.Printf(" - Done [%d messages]\n", len(messages))

	return messages
}

func getMessagesPage(h *hipchat.Client, user *hipchat.User, date string, startIndex int) map[string]*hipchat.Message {
	u := fmt.Sprintf("user/%d/history", user.ID)
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
		// fmt.Printf(" - rate-limited, sleeping for 15s\nGetting conversation with %s", username(user))
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

func writeArchive(users map[string]*hipchat.User, conversations map[string][]*hipchat.Message, filename string) error {
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)

	for userID, conversation := range conversations {
		if len(conversation) == 0 {
			continue
		}

		f, err := w.Create("conversations/" + username(users[userID]) + ".txt")
		if err != nil {
			return err
		}
		for _, day := range pack(conversation) {
			date := day.date.Format("Monday January 2, 2006")
			fmt.Fprintln(f, strings.Repeat(" ", 44), date, strings.Repeat(" ", 74-len(date)))
			fmt.Fprintln(f, strings.Repeat("-", 120))

			for _, usermsgs := range day.msgsByUser {
				fmt.Fprintf(f, "%-30s | %s\n", usermsgs.username, formatmsg(usermsgs.msgs[0]))
				for _, msg := range usermsgs.msgs[1:] {
					fmt.Fprintf(f, "%-30s | %s\n", "", formatmsg(msg))
				}
				fmt.Fprintln(f, strings.Repeat("-", 120))
			}
		}
	}

	f, err := w.Create("machine-readable.json")
	if err != nil {
		return err
	}

	encoded, err := json.MarshalIndent(archive{
		Users:         users,
		Conversations: conversations,
	}, "", "    ")
	if err != nil {
		return err
	}

	_, err = f.Write(encoded)
	if err != nil {
		return err
	}

	err = w.Close()
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filename, buf.Bytes(), 0655)
}

func defaultArchivePath() string {
	home, err := homedir.Dir()
	check(err)
	return path.Join(home, "Documents", "hipchat-archive.zip")
}

type day struct {
	date       time.Time
	msgsByUser []*usermsg
}

type usermsg struct {
	username string
	msgs     []string
}

func pack(messages []*hipchat.Message) []*day {
	var days []*day

	t, err := time.Parse(time.RFC3339Nano, messages[0].Date)
	check(err)

	currentDay := &day{date: t}
	currentMsgs := []*hipchat.Message{}
	for _, msg := range messages {
		t, err := time.Parse(time.RFC3339Nano, msg.Date)
		check(err)

		if t.YearDay() != currentDay.date.YearDay() {
			currentDay.msgsByUser = packmsgs(currentMsgs)
			days = append(days, currentDay)
			currentDay = &day{
				date: t.Truncate(24 * time.Hour),
			}
			currentMsgs = []*hipchat.Message{}
		}

		currentMsgs = append(currentMsgs, msg)
	}

	currentDay.msgsByUser = packmsgs(currentMsgs)
	days = append(days, currentDay)

	return days
}

func packmsgs(messages []*hipchat.Message) []*usermsg {
	if len(messages) == 0 {
		return nil
	}

	var groups []*usermsg

	username := name(messages[0])
	msgs := []string{}
	for _, msg := range messages {
		if n := name(msg); n != username {
			groups = append(groups, &usermsg{
				username: username,
				msgs:     msgs,
			})
			username = n
			msgs = []string{}
		}

		msgs = append(msgs, msg.Message)
	}
	groups = append(groups, &usermsg{
		username: username,
		msgs:     msgs,
	})

	return groups
}

func formatmsg(msg string) string {
	return strings.Join(strings.Split(msg, "\n"), "\n"+strings.Repeat(" ", 30)+" | ")
}

func name(msg *hipchat.Message) string {
	switch from := msg.From.(type) {
	case string:
		return from
	case map[string]interface{}:
		if from["name"].(string) != "" {
			return from["name"].(string)
		}
		return from["mention_name"].(string)
	}

	return ""
}

func username(user *hipchat.User) string {
	if user.Name != "" {
		return user.Name
	}

	return user.MentionName
}

func check(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}
