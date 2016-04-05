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
	"strings"
	"sync"
	"time"

	"github.com/blevesearch/bleve"
	"github.com/blevesearch/bleve/search"
	_ "github.com/blevesearch/bleve/search/highlight/highlighters/ansi"
	"github.com/codegangsta/cli"
	"github.com/mitchellh/go-homedir"
	"github.com/mitchellh/go-wordwrap"
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
					Name:  "archive, a",
					Value: defaultArchivePath(),
					Usage: "Path to the HipChat message archive",
				},
			},
			Action: func(c *cli.Context) {
				if !c.IsSet("token") {
					cli.ShowSubcommandHelp(c)
					return
				}

				archivePath := c.String("filename")
				check(dumpMessages(c.String("token"), archivePath))
				fmt.Println("Archive was written at", archivePath)
			},
		},
		{
			Name:    "index",
			Aliases: []string{"i"},
			Usage:   "Indexes your HipChat private messages archive for fast search",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "archive, a",
					Value: defaultArchivePath(),
					Usage: "Path to the HipChat message archive",
				},
				cli.StringFlag{
					Name:  "index, i",
					Value: defaultIndexPath(),
					Usage: "Path to the HipChat message index",
				},
			},
			Action: func(c *cli.Context) {
				archivePath := c.String("archive")
				indexPath := c.String("index")
				check(indexMessages(archivePath, indexPath))
			},
		},
		{
			Name:    "search",
			Aliases: []string{"s"},
			Usage:   "Search your HipChat private messages archive (must index first)",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "index, i",
					Value: defaultIndexPath(),
					Usage: "Path to the HipChat message index",
				},
			},
			Action: func(c *cli.Context) {
				if c.NArg() < 1 {
					fmt.Println("Usage: hipchat search <query>")
					return
				}
				index, err := bleve.Open(c.String("index"))
				check(err)

				q := bleve.NewQueryStringQuery(strings.Join(c.Args(), " "))
				req := bleve.NewSearchRequest(q)
				req.Fields = []string{"From", "To", "Body", "Date"}
				res, err := index.Search(req)
				check(err)

				if res.Total == 0 {
					fmt.Println("No matches")
					return
				}

				var actualHits search.DocumentMatchCollection
				for _, hit := range res.Hits {
					if hit.Score > 0.5 {
						actualHits = append(actualHits, hit)
					}
				}

				fmt.Printf("%d matche(s), took %s\n", len(actualHits), res.Took)
				for i, hit := range actualHits {
					fmt.Printf("%5d. score: %f\n", i+1, hit.Score)
					fmt.Printf("\tFrom: %s To: %s - %s\n", hit.Fields["From"], hit.Fields["To"], hit.Fields["Date"])
					fmt.Println()
					fmt.Printf("\t%s\n", strings.Join(strings.Split(wordwrap.WrapString(hit.Fields["Body"].(string), 120), "\n"), "\n\t"))
				}
			},
		},
	}

	app.Run(os.Args)
}

func dumpMessages(token, archivePath string) error {
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
	err = os.MkdirAll(path.Dir(archivePath), 0755)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(archivePath, encoded, 0655)
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

func indexMessages(archivePath, indexPath string) error {
	fmt.Print("Reading HipChat archive")
	var hArchive archive
	data, err := ioutil.ReadFile(archivePath)
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, &hArchive)
	if err != nil {
		return err
	}
	fmt.Println(" - Done")

	usersByID := make(map[string]*hipchat.User)
	for i, u := range hArchive.Users {
		usersByID[strconv.Itoa(u.ID)] = &hArchive.Users[i]
	}

	fmt.Print("Creating index")
	os.RemoveAll(indexPath)

	mapping := bleve.NewIndexMapping()
	index, err := bleve.New(indexPath, mapping)
	if err != nil {
		return err
	}
	fmt.Println(" - Done")

	type IndexedMessage struct {
		From []string
		To   []string
		Body string
		Date string
	}

	fmt.Print("Indexing messages")
	var wg sync.WaitGroup
	for recipientID, conversation := range hArchive.Conversations {
		if len(conversation) == 0 {
			continue
		}
		wg.Add(1)
		go func(toID string, messages []*hipchat.Message) {
			defer wg.Done()
			to := usersByID[toID]
			batch := index.NewBatch()

			for _, m := range messages {
				batch.Index(m.ID, IndexedMessage{
					From: names(m.From),
					To:   []string{to.MentionName, to.Name},
					Body: m.Message,
					Date: m.Date,
				})
			}
			check(index.Batch(batch))
		}(recipientID, conversation)
	}
	wg.Wait()
	index.Close()
	fmt.Println(" - Done")

	return nil
}

func names(data interface{}) []string {
	switch from := data.(type) {
	case string:
		return []string{from}
	case map[string]interface{}:
		return []string{from["mention_name"].(string), from["name"].(string)}
	}

	return []string{}
}

func defaultArchivePath() string {
	home, err := homedir.Dir()
	check(err)
	return path.Join(home, ".hipchat", "archive.json")
}

func defaultIndexPath() string {
	home, err := homedir.Dir()
	check(err)
	return path.Join(home, ".hipchat", "index.bleve")
}

func check(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}
