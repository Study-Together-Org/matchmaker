package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/go-redis/redis/v8"
	"github.com/hjson/hjson-go"
	"io/ioutil"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
)

// Variables used for command line parameters
var (
	Token string
)

var ctx = context.Background()

var rdb = redis.NewClient(&redis.Options{
Addr:     "localhost:6379",
Password: "", // no password set
DB:       1,  // use default DB
})

var allQueueNames = make(map[string]int)
var config map[string]interface{}

func init() {
	flag.StringVar(&Token, "t", "", "Bot Token")
	flag.Parse()
}

func check(e error) {
    if e != nil {
        panic(e)
    }
}

func match(s *discordgo.Session, m *discordgo.MessageCreate, queueName string) {
	if value, ok := allQueueNames[queueName]; ok {
		fmt.Println("value: ", value)
	} else {
		_, err := s.ChannelMessageSend(m.ChannelID, m.Author.Mention() + "match target invalid!")
		check(err)
	}

	var capacity int
	if strings.HasPrefix(queueName, "2") {
		capacity = 2
	} else {
		capacity = 3
	}

	var curLen = int(rdb.LLen(ctx, queueName).Val())
	if curLen == capacity - 1 {
		var matchedMembers = rdb.LPopCount(ctx, queueName, curLen).Val()
		_, err := s.ChannelMessageSend(m.ChannelID, queueName + ": " + strings.Join(matchedMembers, " ") + " " + m.Author.Mention())
		check(err)
	} else {
		rdb.RPush(ctx, queueName, m.Author.Mention())
		_, err := s.ChannelMessageSend(m.ChannelID, strconv.Itoa(curLen + 1) + "/" + strconv.Itoa(capacity) + " filled in " + queueName)
		check(err)
	}
}
func main() {
    // read Hjson
	hjsonBytes, err := ioutil.ReadFile(os.Getenv("STUDY_TOGETHER_MODE") + "_config.hjson")
	check(err)

	// We need to provide a variable where Hjson
	// can put the decoded data.

	// Decode and a check for errors.
	if err := hjson.Unmarshal(hjsonBytes, &config); err != nil {
		panic(err)
	}

	fmt.Println(config)

	// Create a new Discord session using the provided bot token.
	dg, err := discordgo.New("Bot " + Token)
	if err != nil {
		fmt.Println("error creating Discord session,", err)
		return
	}

	allQueueNames["2-cam"] = 2
	allQueueNames["2-screenshare"] = 2
	allQueueNames["2-cam-or-screenshare"] = 2

	allQueueNames["3-cam"] = 3
	allQueueNames["3-screenshare"] = 3
	allQueueNames["3-cam-or-screenshare"] = 3
	// Register the messageCreate func as a callback for MessageCreate events.
	dg.AddHandler(messageCreate)

	// Just like the ping pong example, we only care about receiving message
	// events in this example.
	dg.Identify.Intents = discordgo.IntentsGuildMessages

	// Open a websocket connection to Discord and begin listening.
	err = dg.Open()
	if err != nil {
		fmt.Println("error opening connection,", err)
		return
	}

	// Wait here until CTRL-C or other term signal is received.
	fmt.Println("Bot is now running. Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	// Cleanly close down the Discord session.
	dg.Close()
}

// This function will be called (due to AddHandler above) every time a new
// message is created on any channel that the authenticated bot has access to.
//
// It is called whenever a message is created but only when it's sent through a
// server as we did not request IntentsDirectMessages.
func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore all messages created by the bot itself
	// This isn't required in this specific example but it's a good practice.
	if m.Author.ID == s.State.User.ID {
		return
	}

	// Ensure the commands are in command channels
	var found = false
	var commandChannels = config["command_channels"].([]interface{})

	for _, v := range commandChannels{
		if m.ChannelID == v {
			found = true
		}
	}

	if !found {
		_, err := s.ChannelMessageSend(m.ChannelID, m.Author.Mention() + " Please issue commands in bot channels!")
		check(err)
		return
	}

	// In this example, we only care about messages that are "ping".
	if !strings.HasPrefix(m.Content, "%match") {
		return
	}

	var arg = m.Content[len("%match") + 1:] // + 1 for space
	if arg == "help" {
		var helpMsg =
`
Find Study Partners by Preference (Beta and sorry for the primitiveness)
Currently supported preference: 1. choosing between 2 or 3 people and 2. whether to use cam/screenshare
Please use one of the following commands to join the respective queues:

%match 2-cam
%match 2-screenshare
%match 2-cam-or-screenshare
%match 3-cam
%match 3-screenshare
%match 3-cam-or-screenshare
`
		_, err := s.ChannelMessageSend(m.ChannelID, helpMsg)
		check(err)
	}	else {
		match(s, m, arg)
	}
}
