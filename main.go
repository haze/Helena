package main

import (
	"bytes"
	"cloud.google.com/go/vision/apiv1"
	"context"
	"fmt"
	. "github.com/bwmarrin/discordgo"
	"github.com/hvze/helena/twinword"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
)

const (
	// TODO: environment variable token
	SimilarityThreshold float32 = 0.85
	HelenaPrefix                = "HELENA__"
)

var (
	associations map[string]string
)

func associate(with, that string) {
	associations[with] = that
}

func injectAssociations() {
	associations = make(map[string]string)
	// zoe's server
	associate("photo caption", "memes")
	associate("anime", "animation")
}

func similarity(a, b string) float32 {
	return twinword.Similarity(os.Getenv("TWINWORD_KEY"), a, b).Similarity
}

type byConfidence []guess
type guess struct {
	Description string
	Confidence  float32
}

func (s byConfidence) Len() int {
	return len(s)
}
func (s byConfidence) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s byConfidence) Less(i, j int) bool {
	return s[i].Confidence < s[j].Confidence
}

// todo: change type
func getLabels(picture []byte) []guess {
	ctx := context.Background()
	client, err := vision.NewImageAnnotatorClient(ctx)
	if err != nil {
		fmt.Println("Failed to create annotator client, " + err.Error())
		return []guess{}
	}

	image, err := vision.NewImageFromReader(bytes.NewReader(picture))
	report(err)

	annotations, err := client.DetectLabels(ctx, image, nil, 7)
	if len(annotations) == 0 || err != nil {
		fmt.Println("0 guesses or failed to detect labels, " + err.Error())
		return []guess{}
	}
	var labels []guess
	for ind := range annotations {
		anno := annotations[ind]
		labels = append(labels, guess{anno.Description, anno.Confidence})
	}
	sort.Sort(byConfidence(labels))
	return labels
}

func report(e error) {
	if e != nil {
		fmt.Println(e.Error())
		os.Exit(1)
	}
}

func main() {
	// setup discord go
	injectAssociations()
	discord, err := New(os.Getenv("DISCORD_TOKEN"))
	report(err)
	discord.AddHandler(messageHandler)
	err = discord.Open()
	report(err)
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc
	discord.Close()
}

func Join(guesses []guess, sep string) string {
	buf := ""
	for gi := range guesses {
		guess := guesses[gi]
		buf += guess.Description
		if gi != len(guesses)-1 {
			buf += sep + " "
		}
	}
	return buf
}

func hasAssociation(name string) bool {
	_, ok := associations[name]
	return ok
}

func messageHandler(s *Session, m *MessageCreate) {
	if m.Author.ID == os.Getenv("OWNER_ID") {
		if len(m.Attachments) > 0 {
			var movements []string
			tch, err := s.Channel(m.ChannelID)
			report(err)
			guild, err := s.Guild(tch.GuildID)
			report(err)
			for ai := range m.Attachments {
				pic := m.Attachments[ai]
				if strings.HasPrefix(pic.Filename, HelenaPrefix) {
					continue
				}
				req, err := http.Get(pic.URL)
				report(err)
				image, err := ioutil.ReadAll(req.Body)
				report(err)
				req.Body.Close()
				labels := getLabels(image)
				moved := false
				for li := range labels {
					label := labels[li]
					// if the label has a high similarity to the channel name...
					for ci := range guild.Channels {
						if moved {
							break
						}
						ch := guild.Channels[ci]
						if ch.Type != ChannelTypeGuildText {
							continue // save on api calls
						}
						if (hasAssociation(label.Description) && similarity(associations[label.Description], ch.Name) > SimilarityThreshold) || similarity(ch.Name, label.Description) > SimilarityThreshold {
							if ch.ID == m.ChannelID {
								// we posted it in the right channel for once...
								return
							}
							moved = true
							// repost our image
							/* ms := &MessageSend{}
							ms.Content = Join(labels, ",")
							ms.File = &File{Name: HelenaPrefix + pic.Filename, Reader: bytes.NewReader(image)}
							s.ChannelMessageSendComplex(ch.ID, ms) */
							s.ChannelFileSend(m.ChannelID, HelenaPrefix+pic.Filename, bytes.NewReader(image))
							// delete our OG image
							s.ChannelMessageDelete(m.ChannelID, m.ID)
							// post that we moved it
							if len(m.Attachments) == 1 {
								s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("yikes, that image belongs in <#%s>.", ch.ID))
							} else {
								movements = append(movements, "<#"+ch.ID+">")
							}
						}
					}
				}
			}
			if len(movements) > 0 {
				s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("oops, those didn't belong here. they have been moved to their own respective channels, %s.", strings.Join(movements, ", ")))
			}
		}
	}
}
