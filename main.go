package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/bwmarrin/discordgo"
)

func main() {

	// Create a new Discord session
	discord, err := discordgo.New("Bot " + os.Getenv("DISCORD_TOKEN"))
	if err != nil {
		fmt.Println("error creating Discord session, ", err)
		return
	}

	// Register the messageCreate func as a callback for MessageCreate events
	discord.AddHandler(messageCreate)

	//
	discord.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsGuildMessages)

	err = discord.Open()
	if err != nil {
		fmt.Println("error openning connection,", err)
		return
	}

	// Wait until CTRL-C or other termination signal is recieved
	fmt.Println("Bot is running. Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	discord.Close()
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {

	// Ignore all messages created by the bot itself
	if m.Author.ID == s.State.User.ID {
		return
	}
	// Process messages that start with "!ojou " as commands
	if strings.HasPrefix(m.Content, "!ojou ") {
		// [TODO] users should be able to register words
		sentence := string([]rune(m.Content)[6:])
		s.ChannelMessageSend(m.ChannelID, Translate(sentence))
	}
}
