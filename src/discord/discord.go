package discord

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/gateway"
	"github.com/disgoorg/snowflake/v2"

	"github.com/rlko/kuma-disgo/src/config"
	"github.com/rlko/kuma-disgo/src/kuma"
)

var (
	commands = []discord.ApplicationCommandCreate{
		discord.SlashCommandCreate{
			Name:        "status",
			Description: "Get the status of monitored services",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionString{
					Name:        "view",
					Description: "Type of view to display",
					Required:    false,
					Choices: []discord.ApplicationCommandOptionChoiceString{
						{
							Name:  "Minimal",
							Value: "minimal",
						},
						{
							Name:  "Detailed",
							Value: "detailed",
						},
					},
				},
			},
		},
	}

	statusMessageID    snowflake.ID
	statusChannelID    snowflake.ID
	statusMessageMutex sync.Mutex
	statusViewType     string = "minimal"
)

type Bot struct {
	client     bot.Client
	kumaClient *kuma.Client
	cfg        *config.Config
}

func NewBot(token string, kumaClient *kuma.Client, cfg *config.Config) (*Bot, error) {
	client, err := disgo.New(token,
		bot.WithGatewayConfigOpts(
			gateway.WithIntents(
				gateway.IntentGuilds,
				gateway.IntentGuildMessages,
			),
		),
		bot.WithEventListeners(&events.ListenerAdapter{
			OnApplicationCommandInteraction: func(e *events.ApplicationCommandInteractionCreate) {
				if e.Data.CommandName() == "status" {
					handleStatusCommand(e, kumaClient, cfg)
				}
			},
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	return &Bot{
		client:     client,
		kumaClient: kumaClient,
		cfg:        cfg,
	}, nil
}

func (b *Bot) Start(ctx context.Context) error {
	if err := b.client.OpenGateway(ctx); err != nil {
		return fmt.Errorf("failed to connect to gateway: %w", err)
	}

	_, err := b.client.Rest().SetGlobalCommands(b.client.ApplicationID(), commands)
	if err != nil {
		return fmt.Errorf("failed to set global commands: %w", err)
	}

	// Start the update loop
	go b.updateStatusLoop()

	return nil
}

func handleStatusCommand(e *events.ApplicationCommandInteractionCreate, kumaClient *kuma.Client, cfg *config.Config) {
	// Check if user is server owner
	guildID, err := snowflake.Parse(e.GuildID().String())
	if err != nil {
		log.Printf("Failed to parse guild ID: %v", err)
		e.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("Failed to verify permissions.").
			Build(),
		)
		return
	}

	guild, err := e.Client().Rest().GetGuild(guildID, true)
	if err != nil {
		log.Printf("Failed to get guild: %v", err)
		e.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("Failed to verify permissions.").
			Build(),
		)
		return
	}

	if e.User().ID != guild.OwnerID {
		e.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("Only the server owner can use this command.").
			Build(),
		)
		return
	}

	// Get view type from command options
	if viewOpt, ok := e.SlashCommandInteractionData().OptString("view"); ok {
		statusViewType = viewOpt
	}

	// Create or update status message
	embed := createStatusEmbed(kumaClient, cfg)
	if embed == nil {
		e.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("Failed to fetch service statuses.").
			Build(),
		)
		return
	}

	statusMessageMutex.Lock()
	defer statusMessageMutex.Unlock()

	if statusMessageID == 0 {
		// Create new message
		msg, err := e.Client().Rest().CreateMessage(e.Channel().ID(), discord.NewMessageCreateBuilder().
			SetEmbeds(*embed).
			Build(),
		)
		if err != nil {
			log.Printf("Failed to create status message: %v", err)
			e.CreateMessage(discord.NewMessageCreateBuilder().
				SetContent("Failed to create status message.").
				Build(),
			)
			return
		}
		statusMessageID = msg.ID
		statusChannelID = e.Channel().ID()
	} else {
		// Update existing message
		_, err := e.Client().Rest().UpdateMessage(e.Channel().ID(), statusMessageID, discord.NewMessageUpdateBuilder().
			SetEmbeds(*embed).
			Build(),
		)
		if err != nil {
			log.Printf("Failed to update status message: %v", err)
			e.CreateMessage(discord.NewMessageCreateBuilder().
				SetContent("Failed to update status message.").
				Build(),
			)
			return
		}
	}

	e.CreateMessage(discord.NewMessageCreateBuilder().
		SetContent("Status message has been updated.").
		SetEphemeral(true).
		Build(),
	)
}

func (b *Bot) updateStatusLoop() {
	ticker := time.NewTicker(b.cfg.UpdateInterval)
	defer ticker.Stop()

	for range ticker.C {
		statusMessageMutex.Lock()
		if statusMessageID == 0 || statusChannelID == 0 {
			statusMessageMutex.Unlock()
			continue
		}

		embed := createStatusEmbed(b.kumaClient, b.cfg)
		if embed == nil {
			statusMessageMutex.Unlock()
			continue
		}

		_, err := b.client.Rest().UpdateMessage(statusChannelID, statusMessageID, discord.NewMessageUpdateBuilder().
			SetEmbeds(*embed).
			Build(),
		)
		if err != nil {
			log.Printf("Failed to update status message: %v", err)
		}
		statusMessageMutex.Unlock()
	}
}

func createStatusEmbed(kumaClient *kuma.Client, cfg *config.Config) *discord.Embed {
	metrics, err := kumaClient.GetMetrics()
	if err != nil {
		log.Printf("Failed to get metrics: %v", err)
		return nil
	}

	// Determine the worst status for embed color
	worstStatus := 1 // Start with UP (green)
	for _, section := range cfg.Sections {
		for _, service := range section.Services {
			if status, exists := metrics[service.Name]; exists {
				if status.Status < worstStatus {
					worstStatus = status.Status
				}
			}
		}
	}

	// Embed color based on worst status
	var embedColor int
	switch worstStatus {
	case 0: // DOWN
		embedColor = 0xff0000 // Red
	case 2: // PENDING
		embedColor = 0xffff00 // Yellow
	case 3: // MAINTENANCE
		embedColor = 0x0000ff // Blue
	default: // UP or UNKNOWN
		embedColor = 0x00ff00 // Green
	}

	embed := discord.NewEmbedBuilder().
		SetTitle("Service Status").
		SetColor(embedColor).
		SetTimestamp(time.Now())

	for _, section := range cfg.Sections {
		embed.AddField(
			fmt.Sprintf(section.Name),
			"",
			false,
		)

		for _, service := range section.Services {
			status, exists := metrics[service.Name]
			if !exists {
				log.Printf("Service %s not found in metrics", service.Name)
				continue
			}

			var statusEmoji string
			switch status.Status {
			case 1: // UP
				statusEmoji = "🟢"
			case 0: // DOWN
				statusEmoji = "🔴"
			case 2: // PENDING
				statusEmoji = "🟡"
			case 3: // MAINTENANCE
				statusEmoji = "🔵"
			default: // UNKNOWN
				statusEmoji = "⚪"
			}

			// Use display_name if provided, otherwise use name
			displayName := service.DisplayName
			if displayName == "" {
				displayName = service.Name
			}

			var fieldName string
			var description string
			if statusViewType == "minimal" {
				fieldName = fmt.Sprintf("%s %s", statusEmoji, displayName)
				description = ""
			} else {
				fieldName = fmt.Sprintf("%s %s", statusEmoji, displayName)
				description = fmt.Sprintf("type: %s", status.Type)
				if status.URL != "https://" {
					description += fmt.Sprintf("\nurl: %s", status.URL)
				}
				if status.Hostname != "null" {
					description += fmt.Sprintf("\nhost: %s", status.Hostname)
				}
				if status.Port != "null" {
					description += fmt.Sprintf("\nport: %s", status.Port)
				}
			}

			embed.AddField(
				fieldName,
				description,
				false,
			)
		}
	}

	builtEmbed := embed.Build()
	return &builtEmbed
}
