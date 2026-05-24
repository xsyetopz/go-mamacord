package pluginbridge

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/omit"
	"github.com/disgoorg/snowflake/v2"

	"github.com/xsyetopz/go-mamacord/internal/permissions"
	discordplugin "github.com/xsyetopz/go-mamacord/internal/runtime/discord/plugin"
	pluginhost "github.com/xsyetopz/go-mamacord/internal/runtime/plugins"
)

const (
	pluginEventMemberJoin  = "guild_member_join"
	pluginEventMemberLeave = "guild_member_leave"
	pluginEventGuildBan    = "guild_ban"
	pluginEventGuildUnban  = "guild_unban"
)

const (
	defaultPluginAutomationBurst      = 3
	defaultPluginAutomationRatePerSec = 1.0
	defaultPluginAutomationTimeout    = 2 * time.Second
)

type automationDeps struct {
	client                        *bot.Client
	enabledPluginEventSubscribers func(string) []Target
	pluginRoute                   func(string) (Target, bool)
	moduleEnabled                 func(string) bool
	incAutomationFailure          func()
	incPluginFailure              func()
	ensureDMChannel               func(context.Context, uint64) (uint64, error)
}

type Automation struct {
	logger *slog.Logger
	bot    *automationDeps

	limiter *tokenBucketLimiter
}

func NewAutomation(
	logger *slog.Logger,
	client *bot.Client,
	enabledPluginEventSubscribers func(string) []Target,
	pluginRoute func(string) (Target, bool),
	moduleEnabled func(string) bool,
	incAutomationFailure func(),
	incPluginFailure func(),
	ensureDMChannel func(context.Context, uint64) (uint64, error),
) *Automation {
	componentLogger := slog.Default()
	if logger != nil {
		componentLogger = logger.With(slog.String("component", "plugin_automation"))
	}

	return &Automation{
		logger: componentLogger,
		bot: &automationDeps{
			client:                        client,
			enabledPluginEventSubscribers: enabledPluginEventSubscribers,
			pluginRoute:                   pluginRoute,
			moduleEnabled:                 moduleEnabled,
			incAutomationFailure:          incAutomationFailure,
			incPluginFailure:              incPluginFailure,
			ensureDMChannel:               ensureDMChannel,
		},
		limiter: newTokenBucketLimiter(defaultPluginAutomationRatePerSec, defaultPluginAutomationBurst),
	}
}

func (p *Automation) FireEvent(eventName string, payload pluginhost.Payload) {
	eventName = strings.ToLower(strings.TrimSpace(eventName))
	if eventName == "" {
		return
	}

	if p == nil || p.bot == nil {
		return
	}

	if p.bot.enabledPluginEventSubscribers == nil {
		return
	}
	targets := p.bot.enabledPluginEventSubscribers(eventName)
	if len(targets) == 0 {
		return
	}

	go p.fireEvent(context.Background(), targets, eventName, payload)
}

func (p *Automation) fireEvent(
	ctx context.Context,
	targets []Target,
	eventName string,
	payload pluginhost.Payload,
) {
	for _, target := range targets {
		if strings.TrimSpace(target.PluginID) == "" {
			continue
		}
		p.runEventOne(ctx, target, eventName, payload)
	}
}

func (p *Automation) runEventOne(ctx context.Context, target Target, eventName string, payload pluginhost.Payload) {
	callCtx, cancel := context.WithTimeout(ctx, defaultPluginAutomationTimeout)
	defer cancel()

	perms, ok := target.Host.EffectivePermissions(target.PluginID)
	if !ok {
		return
	}

	if !eventAllowed(perms, eventName) {
		p.logger.WarnContext(
			callCtx,
			"plugin event denied by permissions",
			slog.String("plugin", target.PluginID),
			slog.String("event", eventName),
		)
		return
	}

	res, hasValue, err := target.Host.HandleEvent(callCtx, target.PluginID, eventName, payload)
	if err != nil {
		p.incAutomationFailure()
		p.incPluginFailure()
		p.logger.WarnContext(
			callCtx,
			"plugin event failed",
			slog.String("plugin", target.PluginID),
			slog.String("event", eventName),
			slog.String("err", err.Error()),
		)
		return
	}
	if !hasValue {
		return
	}

	actions, parseErr := discordplugin.ParseAutomationActions(res)
	if parseErr != nil {
		p.incAutomationFailure()
		p.incPluginFailure()
		p.logger.WarnContext(
			callCtx,
			"plugin event response invalid",
			slog.String("plugin", target.PluginID),
			slog.String("event", eventName),
			slog.String("err", parseErr.Error()),
		)
		return
	}
	p.executeAutomationActions(callCtx, target.PluginID, perms, payload, actions)
}

func (p *Automation) RunJob(ctx context.Context, job pluginhost.PluginJob) {
	if p == nil || p.bot == nil || p.bot.client == nil || p.bot.pluginRoute == nil || p.bot.moduleEnabled == nil {
		return
	}

	route, ok := p.bot.pluginRoute(job.PluginID)
	if !ok || !p.bot.moduleEnabled(job.PluginID) {
		return
	}

	perms, ok := route.Host.EffectivePermissions(job.PluginID)
	if !ok || !perms.Automation.Jobs {
		return
	}

	for guild := range p.bot.client.Caches.Guilds() {
		guildID := uint64(guild.ID)
		if guildID == 0 {
			continue
		}

		locale := strings.TrimSpace(guild.PreferredLocale)
		if locale == "" {
			locale = discord.LocaleEnglishUS.Code()
		}

		callCtx, cancel := context.WithTimeout(ctx, defaultPluginAutomationTimeout)
		res, hasValue, err := route.Host.HandleJob(callCtx, job.PluginID, job.JobID, pluginhost.Payload{
			GuildID:   snowflake.ID(guildID).String(),
			ChannelID: "",
			UserID:    "",
			Locale:    locale,
			Options: pluginhost.PayloadOptionsFromMap(map[string]any{
				"job_id": job.JobID,
			}),
		})
		cancel()
		if err != nil || !hasValue {
			continue
		}

		actions, parseErr := discordplugin.ParseAutomationActions(res)
		if parseErr != nil {
			p.incAutomationFailure()
			p.incPluginFailure()
			p.logger.WarnContext(
				ctx,
				"plugin job response invalid",
				slog.String("plugin", job.PluginID),
				slog.String("job", job.JobID),
				slog.String("err", parseErr.Error()),
			)
			continue
		}

		p.executeAutomationActions(ctx, job.PluginID, perms, pluginhost.Payload{
			GuildID: snowflake.ID(guildID).String(),
			Locale:  locale,
		}, actions)
	}
}

func eventAllowed(perms permissions.Permissions, eventName string) bool {
	switch eventName {
	case pluginEventMemberJoin, pluginEventMemberLeave:
		return perms.Automation.Events.MemberJoinLeave
	case pluginEventGuildBan, pluginEventGuildUnban:
		return perms.Automation.Events.Moderation
	default:
		return false
	}
}

func (p *Automation) executeAutomationActions(
	ctx context.Context,
	pluginID string,
	perms permissions.Permissions,
	trigger pluginhost.Payload,
	actions []discordplugin.AutomationAction,
) {
	if p == nil || p.bot == nil || p.bot.client == nil {
		return
	}

	for _, a := range actions {
		if !p.allowAutomation(pluginID, trigger, a) {
			p.logger.WarnContext(
				ctx,
				"plugin automation rate-limited",
				slog.String("plugin", pluginID),
				slog.String("type", a.Type),
			)
			continue
		}

		p.executeAutomationAction(ctx, pluginID, perms, trigger, a)
	}
}

func (p *Automation) allowAutomation(pluginID string, trigger pluginhost.Payload, a discordplugin.AutomationAction) bool {
	if p == nil || p.limiter == nil {
		return false
	}
	key := strings.TrimSpace(pluginID) + ":" + strings.TrimSpace(trigger.GuildID) + ":" + strings.TrimSpace(a.Type)
	return p.limiter.Allow(key, time.Now())
}

func (p *Automation) executeAutomationAction(
	ctx context.Context,
	pluginID string,
	perms permissions.Permissions,
	trigger pluginhost.Payload,
	a discordplugin.AutomationAction,
) {
	switch a.Type {
	case "send_channel":
		p.executeSendChannel(ctx, pluginID, perms, trigger, a)
	case "send_dm":
		p.executeSendDM(ctx, pluginID, perms, trigger, a)
	case "timeout_member":
		p.executeTimeoutMember(ctx, pluginID, perms, trigger, a)
	default:
		p.logger.WarnContext(
			ctx,
			"plugin automation unsupported action",
			slog.String("plugin", pluginID),
			slog.String("type", a.Type),
		)
	}
}

func (p *Automation) executeTimeoutMember(
	ctx context.Context,
	pluginID string,
	perms permissions.Permissions,
	trigger pluginhost.Payload,
	a discordplugin.AutomationAction,
) {
	if !perms.Discord.Members {
		p.logger.WarnContext(ctx, "plugin timeout_member denied", slog.String("plugin", pluginID))
		return
	}

	guildID := strings.TrimSpace(a.GuildID)
	if guildID == "" {
		guildID = strings.TrimSpace(trigger.GuildID)
	}
	userID := strings.TrimSpace(a.UserID)
	if userID == "" {
		userID = strings.TrimSpace(trigger.UserID)
	}
	if guildID == "" || userID == "" || a.UntilUnix <= 0 {
		p.logger.WarnContext(ctx, "plugin timeout_member missing fields", slog.String("plugin", pluginID))
		return
	}

	gid, guildErr := snowflake.Parse(guildID)
	uid, userErr := snowflake.Parse(userID)
	if guildErr != nil || userErr != nil {
		p.logger.WarnContext(ctx, "plugin timeout_member invalid ids", slog.String("plugin", pluginID))
		return
	}

	until := time.Unix(a.UntilUnix, 0).UTC()
	if _, err := p.bot.client.Rest.UpdateMember(gid, uid, discord.MemberUpdate{
		CommunicationDisabledUntil: omit.NewPtr(until),
	}); err != nil {
		p.logger.WarnContext(
			ctx,
			"plugin timeout_member failed",
			slog.String("plugin", pluginID),
			slog.String("err", err.Error()),
		)
	}
}

func (p *Automation) executeSendChannel(
	ctx context.Context,
	pluginID string,
	perms permissions.Permissions,
	trigger pluginhost.Payload,
	a discordplugin.AutomationAction,
) {
	if !perms.Discord.Messages {
		p.logger.WarnContext(ctx, "plugin send_channel denied", slog.String("plugin", pluginID))
		return
	}

	channelID := strings.TrimSpace(a.ChannelID)
	if channelID == "" {
		channelID = strings.TrimSpace(trigger.ChannelID)
	}
	if channelID == "" {
		p.logger.WarnContext(ctx, "plugin send_channel missing channel_id", slog.String("plugin", pluginID))
		return
	}

	chID, err := snowflake.Parse(channelID)
	if err != nil {
		p.logger.WarnContext(ctx, "plugin send_channel invalid channel_id", slog.String("plugin", pluginID))
		return
	}

	msg, err := discordplugin.ParseAutomationMessage(pluginID, a.Message)
	if err != nil {
		p.logger.WarnContext(
			ctx,
			"plugin send_channel invalid message",
			slog.String("plugin", pluginID),
			slog.String("err", err.Error()),
		)
		return
	}

	if _, sendErr := p.bot.client.Rest.CreateMessage(chID, msg); sendErr != nil {
		p.logger.WarnContext(
			ctx,
			"plugin send_channel failed",
			slog.String("plugin", pluginID),
			slog.String("err", sendErr.Error()),
		)
	}
}

func (p *Automation) executeSendDM(
	ctx context.Context,
	pluginID string,
	perms permissions.Permissions,
	trigger pluginhost.Payload,
	a discordplugin.AutomationAction,
) {
	if !perms.Discord.Messages {
		p.logger.WarnContext(ctx, "plugin send_dm denied", slog.String("plugin", pluginID))
		return
	}

	userID := strings.TrimSpace(a.UserID)
	if userID == "" {
		userID = strings.TrimSpace(trigger.UserID)
	}
	if userID == "" {
		p.logger.WarnContext(ctx, "plugin send_dm missing user_id", slog.String("plugin", pluginID))
		return
	}

	uid, err := snowflake.Parse(userID)
	if err != nil {
		p.logger.WarnContext(ctx, "plugin send_dm invalid user_id", slog.String("plugin", pluginID))
		return
	}

	msg, err := discordplugin.ParseAutomationMessage(pluginID, a.Message)
	if err != nil {
		p.logger.WarnContext(
			ctx,
			"plugin send_dm invalid message",
			slog.String("plugin", pluginID),
			slog.String("err", err.Error()),
		)
		return
	}

	if p.bot.ensureDMChannel == nil {
		p.logger.WarnContext(ctx, "plugin send_dm missing dm helper", slog.String("plugin", pluginID))
		return
	}
	dmID, dmErr := p.bot.ensureDMChannel(ctx, uint64(uid))
	if dmErr != nil {
		p.logger.WarnContext(
			ctx,
			"plugin send_dm failed to create dm",
			slog.String("plugin", pluginID),
			slog.String("err", dmErr.Error()),
		)
		return
	}

	if _, sendErr := p.bot.client.Rest.CreateMessage(snowflake.ID(dmID), msg); sendErr != nil {
		p.logger.WarnContext(
			ctx,
			"plugin send_dm failed",
			slog.String("plugin", pluginID),
			slog.String("err", sendErr.Error()),
		)
	}
}

type tokenBucketLimiter struct {
	mu sync.Mutex

	ratePerSec float64
	burst      float64
	state      map[string]tokenBucket
}

type tokenBucket struct {
	tokens float64
	last   time.Time
}

func newTokenBucketLimiter(ratePerSec float64, burst int) *tokenBucketLimiter {
	if ratePerSec <= 0 {
		ratePerSec = defaultPluginAutomationRatePerSec
	}
	if burst <= 0 {
		burst = defaultPluginAutomationBurst
	}
	return &tokenBucketLimiter{
		ratePerSec: ratePerSec,
		burst:      float64(burst),
		state:      map[string]tokenBucket{},
	}
}

func (l *tokenBucketLimiter) Allow(key string, now time.Time) bool {
	key = strings.TrimSpace(key)
	if key == "" {
		return false
	}
	if now.IsZero() {
		now = time.Now()
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	b := l.state[key]
	if b.last.IsZero() {
		b = tokenBucket{tokens: l.burst - 1, last: now}
		l.state[key] = b
		return true
	}

	elapsed := now.Sub(b.last).Seconds()
	if elapsed < 0 {
		elapsed = 0
	}
	b.tokens = minFloat(l.burst, b.tokens+elapsed*l.ratePerSec)
	b.last = now
	if b.tokens < 1 {
		l.state[key] = b
		return false
	}
	b.tokens--
	l.state[key] = b
	return true
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func (p *Automation) incAutomationFailure() {
	if p != nil && p.bot != nil && p.bot.incAutomationFailure != nil {
		p.bot.incAutomationFailure()
	}
}

func (p *Automation) incPluginFailure() {
	if p != nil && p.bot != nil && p.bot.incPluginFailure != nil {
		p.bot.incPluginFailure()
	}
}
