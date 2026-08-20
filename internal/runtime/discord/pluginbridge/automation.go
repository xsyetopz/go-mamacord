package pluginbridge

import (
	"context"
	"log/slog"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"

	"github.com/xsyetopz/go-mamacord/internal/permissions"
	pluginhost "github.com/xsyetopz/go-mamacord/internal/runtime/plugins"
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/contract"
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
	maximumConcurrentPluginEvents     = 64
)

type automationDeps struct {
	client                        *bot.Client
	enabledPluginEventSubscribers func(string) []Target
	pluginRoute                   func(string) (Target, bool)
	moduleEnabled                 func(string) bool
	incAutomationFailure          func()
	incPluginFailure              func()
}
type Automation struct {
	logger     *slog.Logger
	bot        *automationDeps
	limiter    *tokenBucketLimiter
	eventSlots chan struct{}
}

func NewAutomation(logger *slog.Logger, client *bot.Client, enabled func(string) []Target, route func(string) (Target, bool), moduleEnabled func(string) bool, incAutomationFailure func(), incPluginFailure func(), ensureDMChannel func(context.Context, uint64) (uint64, error)) *Automation {
	componentLogger := slog.Default()
	if logger != nil {
		componentLogger = logger.With(slog.String("component", "plugin_automation"))
	}
	_ = ensureDMChannel
	return &Automation{logger: componentLogger, bot: &automationDeps{client: client, enabledPluginEventSubscribers: enabled, pluginRoute: route, moduleEnabled: moduleEnabled, incAutomationFailure: incAutomationFailure, incPluginFailure: incPluginFailure}, limiter: newTokenBucketLimiter(defaultPluginAutomationRatePerSec, defaultPluginAutomationBurst), eventSlots: make(chan struct{}, maximumConcurrentPluginEvents)}
}
func (p *Automation) FireEvent(eventName string, invocation contract.Invocation) {
	eventName = strings.ToLower(strings.TrimSpace(eventName))
	if p == nil || p.bot == nil || eventName == "" || p.bot.enabledPluginEventSubscribers == nil {
		return
	}
	targets := p.bot.enabledPluginEventSubscribers(eventName)
	if len(targets) == 0 {
		return
	}
	select {
	case p.eventSlots <- struct{}{}:
		go func() {
			defer func() {
				<-p.eventSlots
				if recovered := recover(); recovered != nil {
					p.logger.Error("plugin event dispatch panicked", "event", eventName, "panic", recovered, "stack", string(debug.Stack()))
					p.failure(context.Background(), "", "plugin event dispatch panicked", nil)
				}
			}()
			p.fireEvent(context.Background(), targets, eventName, invocation)
		}()
	default:
		p.failure(context.Background(), "", "plugin event concurrency limit reached", nil)
	}
}
func (p *Automation) fireEvent(ctx context.Context, targets []Target, eventName string, invocation contract.Invocation) {
	for _, target := range targets {
		p.runEventOne(ctx, target, eventName, invocation)
	}
}
func (p *Automation) runEventOne(ctx context.Context, target Target, eventName string, invocation contract.Invocation) {
	if target.Host == nil || target.PluginID == "" || !p.allow(target.PluginID, idOfInvocationGuild(invocation), eventName) {
		return
	}
	perms, ok := target.Host.EffectivePermissions(target.PluginID)
	if !ok || !eventAllowed(perms, eventName) {
		return
	}
	plans, err := target.Host.PlanEvents(target.PluginID, eventName)
	if err != nil {
		return
	}
	invocation.Kind = contract.InvocationEvent
	if invocation.Event == nil {
		invocation.Event = &contract.EventInput{Name: eventName}
	}
	for _, plan := range plans {
		call := invocation
		call.Route = plan.Route
		callCtx, cancel := context.WithTimeout(ctx, defaultPluginAutomationTimeout)
		terminal, runErr := target.Host.Run(callCtx, target.PluginID, call)
		cancel()
		if runErr != nil {
			p.failure(ctx, target.PluginID, "plugin event failed", runErr)
			continue
		}
		if terminal != nil {
			p.failure(ctx, target.PluginID, "plugin event returned an interaction response", nil)
		}
	}
}
func (p *Automation) RunJob(ctx context.Context, job pluginhost.PluginJob) {
	if p == nil || p.bot == nil || p.bot.client == nil || p.bot.pluginRoute == nil || p.bot.moduleEnabled == nil {
		return
	}
	route, ok := p.bot.pluginRoute(job.PluginID)
	if !ok || route.Host == nil || !p.bot.moduleEnabled(job.PluginID) {
		return
	}
	perms, ok := route.Host.EffectivePermissions(job.PluginID)
	if !ok || !perms.Automation.Jobs {
		return
	}
	plan, err := route.Host.PlanTask(job.PluginID, job.JobID)
	if err != nil {
		return
	}
	for guild := range p.bot.client.Caches.Guilds() {
		guildID := guild.ID.String()
		if !p.allow(job.PluginID, guildID, "job:"+job.JobID) {
			continue
		}
		locale := strings.TrimSpace(guild.PreferredLocale)
		if locale == "" {
			locale = discord.LocaleEnglishUS.Code()
		}
		invocation := contract.Invocation{Route: plan.Route, Kind: contract.InvocationTask, Guild: &contract.GuildRef{ID: guildID, Name: guild.Name}, Locale: locale, Task: &contract.TaskInput{ID: job.JobID}}
		callCtx, cancel := context.WithTimeout(ctx, defaultPluginAutomationTimeout)
		terminal, runErr := route.Host.Run(callCtx, job.PluginID, invocation)
		cancel()
		if runErr != nil {
			p.failure(ctx, job.PluginID, "plugin job failed", runErr)
		} else if terminal != nil {
			p.failure(ctx, job.PluginID, "plugin job returned an interaction response", nil)
		}
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
func (p *Automation) allow(pluginID, guildID, kind string) bool {
	if p == nil || p.limiter == nil {
		return false
	}
	return p.limiter.Allow(pluginID+":"+guildID+":"+kind, time.Now())
}
func (p *Automation) failure(ctx context.Context, pluginID, message string, err error) {
	if p.bot.incAutomationFailure != nil {
		p.bot.incAutomationFailure()
	}
	if p.bot.incPluginFailure != nil {
		p.bot.incPluginFailure()
	}
	attrs := []any{"plugin", pluginID}
	if err != nil {
		attrs = append(attrs, "err", err.Error())
	}
	p.logger.WarnContext(ctx, message, attrs...)
}
func idOfInvocationGuild(invocation contract.Invocation) string {
	if invocation.Guild == nil {
		return ""
	}
	return invocation.Guild.ID
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
