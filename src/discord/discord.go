package discord

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/gateway"
	"github.com/disgoorg/snowflake/v2"

	"github.com/rlko/kuma-disgo/src/config"
	"github.com/rlko/kuma-disgo/src/db"
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
)

type Bot struct {
	client      bot.Client
	kumaClient  *kuma.Client
	cfg         *config.Config
	statusStore *db.StatusStore
}

func NewBot(token string, kumaClient *kuma.Client, cfg *config.Config, statusStore *db.StatusStore) (*Bot, error) {
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
					handleStatusCommand(e, kumaClient, cfg, statusStore)
				}
			},
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	return &Bot{
		client:      client,
		kumaClient:  kumaClient,
		cfg:         cfg,
		statusStore: statusStore,
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

func handleStatusCommand(e *events.ApplicationCommandInteractionCreate, kumaClient *kuma.Client, cfg *config.Config, statusStore *db.StatusStore) {
	// Check if user is server owner
	guildID, err := snowflake.Parse(e.GuildID().String())
	if err != nil {
		log.Printf("Failed to parse guild ID: %v", err)
		e.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("Failed to verify permissions.").
			SetEphemeral(true).
			Build(),
		)
		return
	}

	guild, err := e.Client().Rest().GetGuild(guildID, true)
	if err != nil {
		log.Printf("Failed to get guild: %v", err)
		e.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("Failed to verify permissions.").
			SetEphemeral(true).
			Build(),
		)
		return
	}

	// Debug print: Log server owner name, server name, and channel name
	owner, err := e.Client().Rest().GetUser(guild.OwnerID)
	if err != nil {
		log.Printf("Failed to get owner info: %v", err)
	} else {
		log.Printf("Status command triggered by server owner: %s in server: %s, channel: %s", owner.Username, guild.Name, e.Channel().Name())
	}

	if e.User().ID != guild.OwnerID {
		e.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("Only the server owner can use this command.").
			SetEphemeral(true).
			Build(),
		)
		return
	}

	// Get view type from command options
	viewType := "minimal"
	if viewOpt, ok := e.SlashCommandInteractionData().OptString("view"); ok {
		viewType = viewOpt
	}

	// Create or update status message
	embed := createStatusEmbed(kumaClient, cfg, viewType)
	if embed == nil {
		e.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("Failed to fetch service statuses.").
			SetEphemeral(true).
			Build(),
		)
		return
	}

	// Locking handled by StatusStore
	entries, err := statusStore.GetStatus()
	if err != nil {
		log.Printf("Failed to get status from store: %v", err)
	}

	// Check if there's an existing entry for this channel
	var messageID, channelID string
	for _, entry := range entries {
		if entry.ChannelID == e.Channel().ID().String() {
			messageID = entry.MessageID
			channelID = entry.ChannelID
			break
		}
	}

	if messageID != "" && channelID != "" {
		// Check if the embed post still exists
		_, err := e.Client().Rest().GetMessage(snowflake.MustParse(channelID), snowflake.MustParse(messageID))
		if err != nil {
			log.Printf("Embed post no longer exists for channel %s: %v", channelID, err)
			// Delete the entry from the DB
			if err := statusStore.DeleteStatus(messageID, channelID); err != nil {
				log.Printf("Failed to delete status entry for channel %s: %v", channelID, err)
			}
		} else {
			// Update the existing message
			_, err := e.Client().Rest().UpdateMessage(snowflake.MustParse(channelID), snowflake.MustParse(messageID), discord.NewMessageUpdateBuilder().
				SetEmbeds(*embed).
				Build(),
			)
			if err != nil {
				log.Printf("Failed to update status message: %v", err)
				e.CreateMessage(discord.NewMessageCreateBuilder().
					SetContent("Failed to update status message.").
					SetEphemeral(true).
					Build(),
				)
				return
			}
			statusStore.SetStatus(messageID, channelID, viewType)
			e.CreateMessage(discord.NewMessageCreateBuilder().
				SetContent("Status message has been updated.").
				SetEphemeral(true).
				Build(),
			)
			return
		}
	}

	// If no existing message or it was deleted, create a new one
	err = e.CreateMessage(discord.NewMessageCreateBuilder().
		SetEmbeds(*embed).
		Build(),
	)
	if err != nil {
		log.Printf("Failed to respond to interaction: %v", err)
		return
	}

	// Fetch the original interaction response to get the message ID and channel ID
	msg, err := e.Client().Rest().GetInteractionResponse(e.ApplicationID(), e.Token())
	if err != nil {
		log.Printf("Failed to fetch original interaction response: %v", err)
		return
	}
	statusStore.SetStatus(msg.ID.String(), msg.ChannelID.String(), viewType)
}

func (b *Bot) updateStatusLoop() {
	ticker := time.NewTicker(b.cfg.UpdateInterval)
	defer ticker.Stop()

	log.Printf("Starting status update loop with interval: %v", b.cfg.UpdateInterval)

	for range ticker.C {
		entries, err := b.statusStore.GetStatus()
		if err != nil {
			log.Printf("Failed to get status entries: %v", err)
			continue
		}
		for _, entry := range entries {
			embed := createStatusEmbed(b.kumaClient, b.cfg, entry.ViewType)
			if embed == nil {
				continue
			}
			_, err = b.client.Rest().UpdateMessage(snowflake.MustParse(entry.ChannelID), snowflake.MustParse(entry.MessageID), discord.NewMessageUpdateBuilder().
				SetEmbeds(*embed).
				Build(),
			)
			if err != nil {
				log.Printf("Failed to update status message for channel %s: %v", entry.ChannelID, err)
				// If the embed post doesn't exist anymore, delete the entry from the DB
				if strings.Contains(err.Error(), "Unknown Message") {
					if err := b.statusStore.DeleteStatus(entry.MessageID, entry.ChannelID); err != nil {
						log.Printf("Failed to delete status entry for channel %s: %v", entry.ChannelID, err)
					}
				}
			}
		}
	}
}

func createStatusEmbed(kumaClient *kuma.Client, cfg *config.Config, statusViewType string) *discord.Embed {
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
