package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/chrislgardner/go-discord-bot/hnydiscordgo"
	"github.com/honeycombio/beeline-go"
	"github.com/honeycombio/beeline-go/trace"
	rcon "github.com/katnegermis/pocketmine-rcon"
)

type catFact struct {
	Fact   string
	length int
}

//MessageRespond is the handler for which message respond function should be called
func MessageRespond(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID || !strings.HasPrefix(m.Content, "!") {
		return
	}

	ctx := context.Background()
	var span *trace.Span
	me := hnydiscordgo.MessageEvent{Message: m.Message, Context: ctx}

	ctx, span = hnydiscordgo.StartSpanOrTraceFromMessage(&me, s)

	m.Content = strings.Replace(m.Content, "!", "", 1)
	span.AddField("name", "MessageRespond")

	roles, err := GetMemberRoles(ctx, s, m.Message)
	if err != nil {
		span.AddField("member.role.error", err)
	}

	span.AddField("member.roles", roles)

	if strings.HasPrefix(m.Content, "ping") {
		span.AddField("command", "ping")
		sendResponse(ctx, s, m.ChannelID, "pong")
	} else if strings.HasPrefix(m.Content, "test") {
		span.AddField("command", "test")
		time.Sleep(3 * time.Second)
		sendResponse(ctx, s, m.ChannelID, "test success")
	} else if strings.HasPrefix(m.Content, "split") {
		span.AddField("command", "split")
		str := strings.Split(m.Content, " ")
		sendResponse(ctx, s, m.ChannelID, strings.Join(str[1:], "-"))
	} else if strings.HasPrefix(m.Content, "catfact") {
		span.AddField("command", "catfact")

		enabled := false

		if getFeatureFlagState(ctx, m.Author.ID, roles, "catfact-command") {
			span.AddField("flags.catfact", true)
			enabled = true
		}

		if enabled {
			fact, err := getCatFact(ctx)

			if err != nil {
				span.AddField("error", err)
				sendResponse(ctx, s, m.ChannelID, "error getting cat fact")
			}
			sendResponse(ctx, s, m.ChannelID, fact.Fact)
		} else {
			span.AddField("flags.catfact", false)
			sendResponse(ctx, s, m.ChannelID, "Command not allowed")
		}

	} else if strings.HasPrefix(m.Content, "relationships") {
		span.AddField("command", "relationships")

		s.ChannelMessageSend(m.ChannelID, "some interesting otter things")
	} else if strings.HasPrefix(m.Content, "mc") {
		span.AddField("command", "minecraft")

		enabled := false

		if getFeatureFlagState(ctx, m.Author.ID, roles, "mc-commands") {
			span.AddField("flags.minecraft", true)
			enabled = true
		}

		if enabled {
			resp, err := sendMinecraftCommand(ctx, m.Content)
			if err != nil {
				span.AddField("error", err)
				sendResponse(ctx, s, m.ChannelID, err.Error())
			}

			sendResponse(ctx, s, m.ChannelID, resp)
		} else {
			span.AddField("flags.minecraft", false)
			sendResponse(ctx, s, m.ChannelID, "Command not allowed")
		}
	}

	span.Send()

}

func sendResponse(ctx context.Context, s *discordgo.Session, cid string, m string) {

	ctx, span := beeline.StartSpan(ctx, "send_response")
	defer span.Send()
	beeline.AddField(ctx, "response", m)
	beeline.AddField(ctx, "chennel", cid)

	s.ChannelMessageSend(cid, m)

}

func getCatFact(ctx context.Context) (catFact, error) {

	ctx, span := beeline.StartSpan(ctx, "getCatFact")

	defer span.Send()
	resp, err := http.Get("https://catfact.ninja/fact")
	if err != nil {
		beeline.AddField(ctx, "error", err)
		return catFact{}, err
	}

	defer resp.Body.Close()

	var fact catFact

	err = json.NewDecoder(resp.Body).Decode(&fact)
	if err != nil {
		beeline.AddField(ctx, "error", err)
		return catFact{}, err
	}

	beeline.AddField(ctx, "response", fact.Fact)

	return fact, nil
}

func GetMemberRoles(ctx context.Context, s *discordgo.Session, m *discordgo.Message) ([]string, error) {
	ctx, span := beeline.StartSpan(ctx, "get_discord_role")
	defer span.Send()

	member, err := s.GuildMember(m.GuildID, m.Author.ID)

	if err != nil {
		beeline.AddField(ctx, "error", err)
		return nil, err
	}

	guildRoles, err := s.GuildRoles(m.GuildID)

	if err != nil {
		beeline.AddField(ctx, "error", err)
		return nil, err
	}

	var roles []string

	for _, role := range member.Roles {
		for _, guildRole := range guildRoles {
			if guildRole.ID == role {
				roles = append(roles, guildRole.Name)
			}
		}
	}

	return roles, nil
}

func sendMinecraftCommand(ctx context.Context, comm string) (string, error) {
	ctx, span := beeline.StartSpan(ctx, "minecraft_command")
	defer span.Send()

	conn, err := connectMinecraft(ctx)
	if err != nil {
		beeline.AddField(ctx, "error", err)
		return "", err
	}

	comm = strings.TrimPrefix(comm, "mc ")
	r, err := conn.SendCommand(comm)
	if err != nil {
		beeline.AddField(ctx, "error", err)
		return "", err
	}

	return r, nil

}

func connectMinecraft(ctx context.Context) (*rcon.Connection, error) {
	ctx, span := beeline.StartSpan(ctx, "connect_minecraft")
	defer span.Send()

	addr := os.Getenv("MCSERVERADDR")
	pass := os.Getenv("MCSERVERPASS")

	beeline.AddField(ctx, "mc.server.address", addr)
	conn, err := rcon.NewConnection(addr, pass)

	if err != nil {
		beeline.AddField(ctx, "error", err)
		return nil, err
	}
	return conn, nil
}
