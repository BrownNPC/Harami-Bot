package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/api/cmdroute"
	"github.com/diamondburned/arikawa/v3/api/webhook"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/diamondburned/arikawa/v3/utils/json/option"
)

// To run, do `BOT_TOKEN="TOKEN HERE" go run .`

var commands = []api.CreateCommandData{
	{
		Name:        "troll_rename",
		Description: "epic trolling",

		Options: []discord.CommandOption{
			&discord.UserOption{
				OptionName:  "member",
				Description: "who's name should be changed",
				Required:    true,
			},
			&discord.StringOption{
				OptionName:  "name",
				Description: "what's the new name",
				Required:    true,
			},
		},
	},
	{
		Name:        "troll_impersonate",
		Description: "epic trolling",

		Options: []discord.CommandOption{
			&discord.UserOption{
				OptionName:  "member",
				Description: "who's should I impersonate",
				Required:    true,
			},
			&discord.StringOption{
				OptionName:  "message",
				Description: "what should I say",
				Required:    true,
			},
		},
	},
}

func main() {
	token := os.Getenv("TOKEN")
	if token == "" {
		log.Fatalln("No $BOT_TOKEN given.")
	}

	h := newHandler(state.New("Bot " + token))
	h.s.AddInteractionHandler(h)

	h.s.AddIntents(gateway.IntentGuilds)
	h.s.AddIntents(gateway.IntentGuildMembers)
	h.s.AddHandler(func(*gateway.ReadyEvent) {
		me, _ := h.s.Me()
		log.Println("connected to the gateway as", me.Tag())
	})

	if err := cmdroute.OverwriteCommands(h.s, commands); err != nil {
		log.Fatalln("cannot update commands:", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if err := h.s.Connect(ctx); err != nil {
		log.Fatalln("cannot connect:", err)
	}
}

type handler struct {
	*cmdroute.Router
	s *state.State
}

func newHandler(s *state.State) *handler {
	h := &handler{s: s}

	h.Router = cmdroute.NewRouter()
	// Automatically defer handles if they're slow.
	h.Use(cmdroute.Deferrable(s, cmdroute.DeferOpts{}))
	h.AddFunc("troll_rename", h.TrollRename)
	h.AddFunc("troll_impersonate", h.TrollImpersonate)

	return h
}

func (h *handler) TrollRename(ctx context.Context, data cmdroute.CommandData) *api.InteractionResponseData {
	var options struct {
		Name string         `discord:"name"`
		User discord.UserID `discord:"member"`
	}
	if err := data.Options.Unmarshal(&options); err != nil {
		return errorResponse(err)
	}
	newNick := api.ModifyMemberData{
		Nick: &options.Name,
	}
	err := h.s.ModifyMember(data.Event.GuildID, options.User, newNick)
	if err != nil {
		log.Println("failed to rename:", err)
		return errorResponse(err)
	}
	return &api.InteractionResponseData{
		Content:         option.NewNullableString(fmt.Sprint("set the nickname to ", options.Name)),
		AllowedMentions: &api.AllowedMentions{}, // don't mention anyone
		Flags:           discord.EphemeralMessage,
	}

}

func (h *handler) TrollImpersonate(ctx context.Context, data cmdroute.CommandData) *api.InteractionResponseData {
	var options struct {
		Message string         `discord:"message"`
		User    discord.UserID `discord:"member"`
	}
	if err := data.Options.Unmarshal(&options); err != nil {
		return errorResponse(err)
	}
	member, err := h.s.Member(data.Event.GuildID, options.User)
	if err != nil {
		log.Println("failed to get member:", err)
		return errorResponse(err)
	}
	go func() {

		wh, err := h.s.CreateWebhook(data.Event.ChannelID, api.CreateWebhookData{
			Name: "troll",
		})
		if err != nil {
			log.Println("failed to create webhook:", err)
		}
		defer h.s.DeleteWebhook(wh.ID)
		client, err := webhook.NewFromURL(wh.URL)
		if err != nil {
			log.Println("failed to create webhook client:", err)
		}
		client.Execute(
			webhook.ExecuteData{
				Content:         options.Message,
				AllowedMentions: &api.AllowedMentions{}, // don't mention anyone
				Username:        member.User.DisplayName,
				AvatarURL:       member.User.AvatarURL(),
			},
		)

	}()
	return &api.InteractionResponseData{
		Content:         option.NewNullableString("done"),
		AllowedMentions: &api.AllowedMentions{}, // don't mention anyone
		Flags:           discord.EphemeralMessage,
	}

}

func errorResponse(err error) *api.InteractionResponseData {
	return &api.InteractionResponseData{
		Content:         option.NewNullableString("**Error:** " + err.Error()),
		Flags:           discord.EphemeralMessage,
		AllowedMentions: &api.AllowedMentions{ /* none */ },
	}
}
