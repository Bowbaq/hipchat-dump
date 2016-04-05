package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/tbruyelle/hipchat-go/hipchat"
)

var (
	token    = flag.String("token", "", "HipChat auth token with view_group, view_messages scope - see https://www.hipchat.com/account/api")
	filename = flag.String("filename", "conversations.json", "File where the conversations will be dumped")
)

func main() {
	flag.Parse()

	if *token == "" {
		flag.PrintDefaults()
	}

	h := hipchat.NewClient(*token)

	var users []hipchat.User
	users, res, err := h.User.List(&hipchat.UserListOptions{
		ListOptions: hipchat.ListOptions{
			MaxResults: 1000,
		},
	})
	for res.StatusCode == 429 {
		fmt.Println("Rate limited, sleeping for 15s")
		time.Sleep(15 * time.Second)
		users, res, err = h.User.List(&hipchat.UserListOptions{
			ListOptions: hipchat.ListOptions{
				MaxResults: 1000,
			},
		})
	}
	check(err)

	conversations := make(map[string][]*hipchat.Message)
	for _, user := range users {
		conversations[strconv.Itoa(user.ID)] = getMessages(h, user.ID)
	}

	encoded, err := json.MarshalIndent(conversations, "", "    ")
	check(err)

	check(ioutil.WriteFile(*filename, encoded, 0655))
}

type HistoryViewOptions struct {
	hipchat.ListOptions

	Reverse bool
}

func getMessages(h *hipchat.Client, userID int) []*hipchat.Message {
	uniqueMessages := getMessagesPage(h, userID, "recent", 0)
	if len(uniqueMessages) == 0 {
		return []*hipchat.Message{}
	}

	now := time.Now().Add(-1 * time.Minute).UTC().Format(time.RFC3339)
	start := 0
	for {
		page := getMessagesPage(h, userID, now, start)
		for _, msg := range page {
			uniqueMessages[msg.ID] = msg
		}

		if len(page) < pageSize {
			break
		}

		start += len(page) - 1
	}

	var messages []*hipchat.Message
	for _, msg := range uniqueMessages {
		messages = append(messages, msg)
	}
	sort.Sort(byMostRecent(messages))

	return messages
}

var pageSize = 1000

func getMessagesPage(h *hipchat.Client, userID int, date string, startIndex int) map[string]*hipchat.Message {
	u := fmt.Sprintf("user/%d/history", userID)
	opt := &hipchat.HistoryOptions{
		ListOptions: hipchat.ListOptions{
			MaxResults: pageSize,
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
	for res.StatusCode == 429 {
		fmt.Println("Rate limited, sleeping for 15s")
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

func check(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}

type byMostRecent []*hipchat.Message

func (msgs byMostRecent) Len() int           { return len(msgs) }
func (msgs byMostRecent) Less(i, j int) bool { return msgs[i].Date > msgs[j].Date }
func (msgs byMostRecent) Swap(i, j int)      { msgs[i], msgs[j] = msgs[j], msgs[i] }
