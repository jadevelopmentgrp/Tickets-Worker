package listeners

import (
	"fmt"
	"github.com/TicketsBot/common/permission"
	"github.com/TicketsBot/common/premium"
	"github.com/TicketsBot/common/sentry"
	translations "github.com/TicketsBot/database/translations"
	"github.com/TicketsBot/worker"
	"github.com/TicketsBot/worker/bot/dbclient"
	"github.com/TicketsBot/worker/bot/errorcontext"
	"github.com/TicketsBot/worker/bot/logic"
	"github.com/TicketsBot/worker/bot/utils"
	"github.com/rxdn/gdl/objects/interaction"
)

func OnClaimReact(worker *worker.Context, data interaction.ButtonInteraction) {
	if data.Member == nil {
		return
	}

	errorCtx := errorcontext.WorkerErrorContext{
		Guild:   data.GuildId.Value,
		User:    data.Member.User.Id,
		Channel: data.ChannelId,
	}

	// TODO: Create a button context
	premiumTier := utils.PremiumClient.GetTierByGuildId(data.GuildId.Value, true, worker.Token, worker.RateLimiter)

	// Get permission level
	permissionLevel, err := permission.GetPermissionLevel(utils.ToRetriever(worker), *data.Member, data.GuildId.Value)
	if err != nil {
		sentry.ErrorWithContext(err, errorCtx)
		return
	}

	if permissionLevel < permission.Support {
		utils.SendEmbed(worker, data.ChannelId, data.GuildId.Value, nil, utils.Red, "Error", translations.MessageCloseNoPermission, nil, 30, premiumTier > premium.None)
		//ctx.Reply(utils.Red, "Error", translations.MessageNoPermission)
		return
	}

	// Get ticket struct
	ticket, err := dbclient.Client.Tickets.GetByChannel(data.ChannelId); if err != nil {
		sentry.ErrorWithContext(err, errorCtx)
		return
	}

	// Verify this is a ticket channel
	if ticket.UserId == 0 {
		utils.SendEmbed(worker, data.ChannelId, data.GuildId.Value, nil, utils.Red, "Error", translations.MessageNotATicketChannel, nil, 30, premiumTier > premium.None)
		//ctx.Reply(utils.Red, "Error", translations.MessageNotATicketChannel)
		return
	}

	if err := logic.ClaimTicket(worker, ticket, data.Member.User.Id); err != nil {
		sentry.ErrorWithContext(err, errorCtx)
		//ctx.HandleError(err)
		return
	}

	utils.SendEmbed(worker, data.ChannelId, data.GuildId.Value, nil, utils.Green, "Ticket Claimed", translations.MessageClaimed, nil, 30, premiumTier > premium.None, fmt.Sprintf("<@%d>", data.Member.User.Id))
}